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

It additionally asserts the **nested-module topology** — `contrib/*` (ADR-0003,
ADR-0040) and `examples/*` (ADR-0054). Those directories are separate modules,
and that boundary is the only thing keeping their dependencies out of the core's
graph. Nothing else here would notice if one lost its `go.mod`, because every
other check asks `go list ./...` what the module contains and the answer would
simply have grown. Read from the filesystem, this check runs first and
short-circuits; it also refuses a `replace`, a committed `go.work`, and a core
requirement pinned to anything but a released tag, none of which breaks a build.

Run it from the repository root:

    python tools/import_graph_lint.py

Exit status is 0 when every invariant holds and 1 otherwise, with each violation
named. Adding a dependency or an internal edge means changing the allowlists here
*and* the depguard rules, which is the point: both edits land in the PR that needs
the ADR.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys

# REPO is the repository path; MODULE carries the major suffix. They differ from v2 on,
# and the difference matters: the core's packages live under MODULE, while contrib/*
# submodules are versioned independently (ADR-0040) and keep repository-rooted paths with
# no major suffix of the core's.
REPO = "github.com/danielPoloWork/egl-utils-go"
MODULE = REPO + "/v2"

# SRC_ROOT is the in-repo source tree the core's packages sit under. Stripping it as well
# as MODULE keeps every allowlist below written in short package names ("cache") rather
# than full paths, which is what makes them readable and diffable.
SRC_ROOT = "pkg"


def short_pkg(import_path: str) -> str:
    """A package's short name: import path minus the module and the source root."""
    rest = import_path[len(MODULE) :].lstrip("/") if import_path.startswith(MODULE) else import_path
    if rest == SRC_ROOT:
        return "(root)"
    if rest.startswith(SRC_ROOT + "/"):
        rest = rest[len(SRC_ROOT) + 1 :]
    return rest or "(root)"

# ---------------------------------------------------------------------------
# ADR-0004 ring 2 and 3: every non-stdlib runtime module, and the single package
# whose ADR justified it. A module absent from this map may not be imported by
# production code at all.
# ---------------------------------------------------------------------------
RUNTIME_DEPS = {
    "gopkg.in/yaml.v3": ("config", "ADR-0018 - the YAML parser for config.Load"),
    "golang.org/x/crypto": ("hash", "ADR-0024 - bcrypt password hashing"),
    "golang.org/x/sync": ("semaphore", "ADR-0009 - the weighted semaphore"),
}

# ADR-0004's test-only ring: never imported by production code.
#
# ADR-0050 removed both Prometheus modules - client_golang from this ring's
# runtime counterpart above, and client_model from here, since the metrics tests
# no longer decode protobuf exposition structures. Nine modules left the graph
# with them, so the deny rule in .golangci.yml now covers github.com/prometheus
# with no exception at all rather than confining it to one package.
TEST_ONLY_DEPS = {
    "github.com/stretchr/testify",
    "go.uber.org/goleak",
    "pgregory.net/rapid",
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

# ---------------------------------------------------------------------------
# Directories whose immediate children are separate modules. Each maps to the
# ADRs that decided the boundary and to what joining the root module would cost,
# because the failure is the same in both cases and only the cost differs: a
# directory of Go files with no go.mod of its own is silently part of the ROOT
# module, and everything it imports joins the core's dependency graph while every
# `go list ./...`-based check keeps passing.
# ---------------------------------------------------------------------------
NESTED_MODULE_PARENTS = {
    "contrib": (
        "ADR-0003, ADR-0040",
        "its driver dependencies would enter the core's graph",
    ),
    "examples": (
        "ADR-0054",
        "everything the showcase imports would enter the core's graph",
    ),
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
        pkg = short_pkg(pkg_path)

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
                target = short_pkg(imp)
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
        pkg = short_pkg(pkg_path)
        for imp in imports.split():
            if imp.startswith(MODULE + "/"):
                actual.add((pkg, short_pkg(imp)))
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


def unmoduled_go_file(top: str) -> str | None:
    """The first Go file under `top` that belongs to no module of its own.

    Recursive rather than a listing of `top`, because the failure is the same one
    directory deeper: `examples/foo/cmd/server/main.go` with no
    `examples/foo/go.mod` above it still joins the ROOT module. Any directory
    carrying its own go.mod is pruned — what is inside it is that module's
    business, not the root's.
    """
    for dirpath, dirnames, filenames in os.walk(top):
        dirnames.sort()
        if dirpath != top and "go.mod" in filenames:
            dirnames[:] = []
            continue
        for fn in sorted(filenames):
            if fn.endswith(".go"):
                return os.path.join(dirpath, fn).replace(os.sep, "/")
    return None


def check_nested_modules(problems: list[str]) -> None:
    """Every contrib/* and examples/* package directory must carry its own go.mod.

    This is the load-bearing check for both nested-module topologies (ADR-0003 and
    ADR-0040 for contrib, ADR-0054 for examples). They exist for opposite-looking
    reasons — contrib deliberately imports drivers the core is forbidden to touch,
    examples deliberately imports many feature packages at once and may grow a
    dependency of its own — but the mechanism protecting the core is identical and
    it is not the lint config: it is that each directory is a *separate module*,
    which `go list ./...` does not descend into.

    Delete or rename one of those go.mod files and the failure is silent: the .go
    files underneath simply join the root module, whatever they import joins the
    core's dependency graph, and every other check here keeps passing because it
    asks `go list ./...` what the module contains. So this check looks at the
    filesystem instead.
    """
    for parent in sorted(NESTED_MODULE_PARENTS):
        adrs, consequence = NESTED_MODULE_PARENTS[parent]
        if not os.path.isdir(parent):
            continue
        for name in sorted(os.listdir(parent)):
            d = os.path.join(parent, name)
            if not os.path.isdir(d):
                continue
            mod = os.path.join(d, "go.mod")
            if not os.path.isfile(mod):
                stray = unmoduled_go_file(d)
                if stray is None:
                    continue
                problems.append(
                    "%s holds Go files (%s) but no go.mod, so they are part of the ROOT "
                    "module -- %s. Every %s/* package is a separate module (%s)."
                    % (d, stray, consequence, parent, adrs)
                )
                continue
            # A nested module carries its OWN major version, never the core's, so
            # the expected path has no /vN suffix (ADR-0040's per-module rule).
            want = "%s/%s/%s" % (REPO, parent, name)
            with open(mod, encoding="utf-8") as fh:
                first = fh.readline().strip()
            if first != "module " + want:
                problems.append(
                    "%s declares %r but its directory implies module path %r; a mismatch "
                    "breaks `go get` for that submodule." % (mod, first, want)
                )


def check_nested_modules_resolve_like_a_consumer(problems: list[str]) -> None:
    """No `replace`, no workspace, and the core required at a released version.

    ADR-0040 decided this for contrib and ADR-0054 repeats it for examples, and
    until now it was intended rather than enforced. Both rejected alternatives
    fail the same way — silently, with CI green:

      * a `replace … => ../..` builds the nested module against the working tree,
        so the `require` line consumers actually resolve is never exercised;
      * a committed `go.work` switches every root-level `go build`, `go vet`,
        `golangci-lint` and `govulncheck` invocation into workspace mode, which
        changes resolution for the jobs that currently pass;
      * a pseudo-version (`go get …@master`) means the module is built against a
        commit no consumer can `go get` by name.

    None of these breaks a build, which is exactly why they need a gate.
    """
    for workspace in ("go.work", "go.work.sum"):
        if os.path.isfile(workspace):
            problems.append(
                "%s exists in the repository root. A committed workspace switches every "
                "root-level go/golangci-lint/govulncheck invocation into workspace mode "
                "and changes dependency resolution for the whole module (ADR-0040 "
                "rejected it explicitly). Nested modules build against the released "
                "core instead." % workspace
            )

    for rel in nested_modules():
        mod_file = os.path.join(rel, "go.mod")
        mod = json.loads(run("go", "mod", "edit", "-json", mod_file))

        for rep in mod.get("Replace") or []:
            problems.append(
                "%s has a replace directive (%s => %s). A nested module builds against the "
                "RELEASED core so it is tested the way a consumer gets it; replace is "
                "ignored for anyone who depends on the module, so it would validate a "
                "configuration nobody receives (ADR-0040, ADR-0054)."
                % (mod_file, rep["Old"]["Path"], rep["New"]["Path"])
            )

        for req in mod.get("Require") or []:
            if req["Path"] != MODULE:
                continue
            # A pseudo-version carries a timestamp-and-hash build suffix; a released
            # tag does not. `vX.Y.Z` (with an optional pre-release) is a tag.
            if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?", req["Version"]) or re.search(
                r"-\d{14}-[0-9a-f]{12}$", req["Version"]
            ):
                problems.append(
                    "%s requires %s at %s, which is not a released version. Nested modules "
                    "require the core at a tag so CI resolves exactly what `go get` will "
                    "(ADR-0040, ADR-0054)." % (mod_file, MODULE, req["Version"])
                )


def nested_modules() -> list[str]:
    """Every nested module in the repository, as `<parent>/<name>` paths."""
    found = []
    for parent in sorted(NESTED_MODULE_PARENTS):
        if not os.path.isdir(parent):
            continue
        for name in sorted(os.listdir(parent)):
            if os.path.isfile(os.path.join(parent, name, "go.mod")):
                found.append("%s/%s" % (parent, name))
    return found


def report(problems: list[str]) -> int:
    print("Import-graph lint: %d violation(s)\n" % len(problems))
    for p in problems:
        print("  [!]   %s" % p)
    print(
        "\nPolicy: docs/adr/0004-runtime-dependency-policy.md (dependency rings),\n"
        "        docs/adr/0035-import-graph-enforcement.md (this gate),\n"
        "        docs/adr/0033-config-struct-validation.md (the one internal edge),\n"
        "        docs/adr/0040-contrib-submodules.md (the contrib topology),\n"
        "        docs/adr/0054-examples-service-module.md (the examples topology)."
    )
    return 1


def main() -> int:
    # The nested-module check runs first and short-circuits, because it is the one
    # failure that breaks the tools underneath it. A contrib or examples directory
    # without a go.mod joins the root module, and `go list ./...` then dies on an
    # import it cannot resolve -- an opaque "no required module provides package"
    # error rather than the real problem. Diagnosing first, then measuring.
    problems: list[str] = []
    check_nested_modules(problems)
    if problems:
        return report(problems)

    check_nested_modules_resolve_like_a_consumer(problems)
    check_direct_requirements(problems)
    check_package_imports(problems)
    check_edges_are_reachable(problems)
    check_module_graph(problems)

    if problems:
        return report(problems)

    nested = nested_modules()
    print(
        "Import-graph lint: OK - %d runtime deps in their owning packages, "
        "%d sanctioned internal edge(s), no test-only deps in production, "
        "%d nested module(s) held outside the core graph%s."
        % (
            len(RUNTIME_DEPS),
            len(ALLOWED_INTERNAL_EDGES),
            len(nested),
            (" (" + ", ".join(nested) + ")") if nested else "",
        )
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
