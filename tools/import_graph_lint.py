#!/usr/bin/env python3
"""Assert the module's dependency rings and internal import graph.

The written policies are ADR-0004 (three dependency rings, two vetted third-party
runtime entries) and spec v2 §3 (the layered internal import graph: arrows point
downward only, and a same-layer edge is legal only where the spec mandates the
composition). `.golangci.yml`'s depguard rules enforce them per file; this tool
enforces them over the *resolved* graph, which catches three things depguard
cannot:

  * a new **direct module requirement** in go.mod, which is a policy decision
    rather than an import (depguard only ever sees imports);
  * a **blank import** of a sibling package — verified: depguard does not report
    `import _ "…/sibling"` even though it reports blank imports of external
    modules, so without this check `_` would be a bypass (revive's blank-imports
    rule also objects, but that is a style rule, not the architecture);
  * drift between the two files, since both must agree for CI to pass.

Run it from the repository root:

    python tools/import_graph_lint.py

Exit status is 0 when every invariant holds and 1 otherwise, with each violation
named. Adding a dependency or an internal edge means changing the allowlists here
*and* the depguard rules, which is the point: both edits land in the PR that needs
the ADR.
"""

from __future__ import annotations

import json
import subprocess
import sys

MODULE = "github.com/danielPoloWork/egl-utils-go"

# ---------------------------------------------------------------------------
# ADR-0004 ring 2 and 3: every non-stdlib runtime module, and the single package
# whose ADR justified it. A module absent from this map may not be imported by
# production code at all.
# ---------------------------------------------------------------------------
RUNTIME_DEPS = {
    "gopkg.in/yaml.v3": ("config", "ADR-0018 - the YAML parser for config.Load"),
    "github.com/prometheus/client_golang": ("metrics", "ADR-0027 - Prometheus exposition"),
    "golang.org/x/crypto": ("hash", "ADR-0024 - bcrypt password hashing"),
    "golang.org/x/sync": ("semaphore", "ADR-0009 - the weighted semaphore"),
}

# ADR-0004's test-only ring: never imported by production code. client_model is a
# direct requirement because the metrics tests assert on decoded exposition
# structures (dto) rather than matching text (ADR-0027).
TEST_ONLY_DEPS = {
    "github.com/stretchr/testify",
    "go.uber.org/goleak",
    "pgregory.net/rapid",
    "github.com/prometheus/client_model",
}

# ---------------------------------------------------------------------------
# Spec section 3: the internal graph. Exactly one same-layer edge is sanctioned -
# config -> validator, because spec item 13 defines config as "configuration with
# struct validation (via item 19)" (ADR-0033). Every other feature package is
# self-contained, which is what keeps them independently adoptable.
# ---------------------------------------------------------------------------
ALLOWED_INTERNAL_EDGES = {
    ("config", "validator"),
}


def run(*args: str) -> str:
    proc = subprocess.run(args, capture_output=True, text=True, check=False)
    if proc.returncode != 0:
        sys.exit("failed to run %s:\n%s" % (" ".join(args), proc.stderr.strip()))
    return proc.stdout


def module_prefix(import_path: str) -> str | None:
    """Return the governed module prefix an import belongs to, if any."""
    for prefix in RUNTIME_DEPS:
        if import_path == prefix or import_path.startswith(prefix + "/"):
            return prefix
    return None


def check_direct_requirements(problems: list[str]) -> None:
    """Every direct requirement in go.mod must be a named ring member."""
    mod = json.loads(run("go", "mod", "edit", "-json"))
    allowed = set(RUNTIME_DEPS) | TEST_ONLY_DEPS
    for req in mod.get("Require") or []:
        if req.get("Indirect"):
            continue
        path = req["Path"]
        if path not in allowed:
            problems.append(
                "go.mod requires %s directly, which is outside the ADR-0004 rings.\n"
                "        A new dependency needs a superseding ADR before the import lands;\n"
                "        then add it to RUNTIME_DEPS or TEST_ONLY_DEPS here and to the\n"
                "        depguard rules in .golangci.yml." % path
            )


def check_package_imports(problems: list[str]) -> None:
    """Production imports: governed modules stay in their package; internal edges
    stay within the sanctioned set."""
    # -deps is omitted deliberately: this asserts what each package imports
    # directly, which is the decision under review. Transitive dependencies of a
    # budgeted module are a consequence of that budget, not a separate choice, and
    # pinning them here would turn every upstream bump into a CI failure.
    out = run("go", "list", "-f", "{{.ImportPath}}\t{{join .Imports \" \"}}", "./...")
    for line in out.splitlines():
        if not line.strip():
            continue
        pkg_path, _, imports = line.partition("\t")
        pkg = pkg_path[len(MODULE) :].lstrip("/") or "(root)"

        for imp in imports.split():
            # Ring 2/3: a governed module may only be imported by its owner.
            prefix = module_prefix(imp)
            if prefix is not None:
                owner, why = RUNTIME_DEPS[prefix]
                if pkg != owner:
                    problems.append(
                        "%s imports %s, but %s is budgeted for the %s package only (%s)."
                        % (pkg, imp, prefix, owner, why)
                    )

            # Test-only ring must never appear in production imports.
            for test_dep in TEST_ONLY_DEPS:
                if imp == test_dep or imp.startswith(test_dep + "/"):
                    problems.append(
                        "%s imports the test-only dependency %s from production code "
                        "(ADR-0004)." % (pkg, imp)
                    )

            # Spec section 3: internal edges.
            if imp == MODULE or imp.startswith(MODULE + "/"):
                target = imp[len(MODULE) :].lstrip("/") or "(root)"
                if (pkg, target) not in ALLOWED_INTERNAL_EDGES:
                    problems.append(
                        "internal edge %s -> %s is not sanctioned (spec section 3, ADR-0033).\n"
                        "        Feature packages do not import each other; the only "
                        "sanctioned edge is config -> validator.\n"
                        "        Compose in the consumer, or add an ADR and list the edge "
                        "in ALLOWED_INTERNAL_EDGES." % (pkg, target)
                    )


def check_edges_are_reachable(problems: list[str]) -> None:
    """A sanctioned edge that no longer exists means the allowlist is stale - the
    exception outlived the composition that justified it."""
    out = run("go", "list", "-f", "{{.ImportPath}}\t{{join .Imports \" \"}}", "./...")
    actual = set()
    for line in out.splitlines():
        pkg_path, _, imports = line.partition("\t")
        pkg = pkg_path[len(MODULE) :].lstrip("/") or "(root)"
        for imp in imports.split():
            if imp.startswith(MODULE + "/"):
                actual.add((pkg, imp[len(MODULE) :].lstrip("/")))
    for edge in sorted(ALLOWED_INTERNAL_EDGES - actual):
        problems.append(
            "sanctioned internal edge %s -> %s no longer exists; drop it from "
            "ALLOWED_INTERNAL_EDGES (and its depguard exception) so the policy does "
            "not carry a dead exception." % edge
        )


def check_module_graph(problems: list[str]) -> None:
    """Cross-check with `go mod graph`: every module our own module depends on must
    be declared in go.mod, so the graph and the manifest cannot drift."""
    declared = {
        req["Path"] for req in (json.loads(run("go", "mod", "edit", "-json")).get("Require") or [])
    }
    for line in run("go", "mod", "graph").splitlines():
        parts = line.split()
        if len(parts) != 2:
            continue
        source, target = parts
        if source != MODULE:  # only edges out of our own module
            continue
        target_path = target.split("@", 1)[0]
        # `go` and `toolchain` are pseudo-modules the graph uses to record the
        # language and toolchain versions; they are directives in go.mod, not
        # requirements, so they never appear in the Require list.
        if target_path in ("go", "toolchain"):
            continue
        if target_path not in declared:
            problems.append(
                "go mod graph has edge %s -> %s but go.mod does not require it; "
                "run `go mod tidy`." % (MODULE, target_path)
            )


def main() -> int:
    problems: list[str] = []
    check_direct_requirements(problems)
    check_package_imports(problems)
    check_edges_are_reachable(problems)
    check_module_graph(problems)

    if problems:
        print("Import-graph lint: %d violation(s)\n" % len(problems))
        for p in problems:
            print("  [!]   %s" % p)
        print(
            "\nPolicy: docs/adr/0004-runtime-dependency-policy.md (dependency rings),\n"
            "        docs/adr/0035-import-graph-enforcement.md (this gate),\n"
            "        docs/adr/0033-config-struct-validation.md (the one internal edge)."
        )
        return 1

    print(
        "Import-graph lint: OK - %d runtime deps in their owning packages, "
        "%d sanctioned internal edge(s), no test-only deps in production."
        % (len(RUNTIME_DEPS), len(ALLOWED_INTERNAL_EDGES))
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
