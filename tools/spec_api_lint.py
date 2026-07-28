#!/usr/bin/env python3
"""Assert that spec section 5 and the module's real exported surface agree.

Spec section 5 enumerates the public interface, and its closing clause binds
SemVer to the module's whole exported surface (ROADMAP 12.1). Those two facts
make the enumeration load-bearing: a reader treats it as the API, and until
12.1 it had silently fallen twelve identifiers behind what v1.1.0 shipped.
Nothing caught that. `consistency_lint.py`'s spec-map gate verifies that every
spec section has a fulfilling roadmap item -- never that the section's *claims*
are true -- which is how nine divergences accumulated across ten milestones
without a red build.

Run it from the repository root:

    python tools/spec_api_lint.py

It fails in BOTH directions, because the two failures are different bugs:

  * **shipped but unlisted** -- an exported identifier missing from section 5.
    This is how Milestone 10's additions accumulated: each PR added public API
    and none updated the spec.
  * **listed but gone** -- section 5 names something the module no longer
    exports. A stale promise is worse than a missing one, because a consumer
    can write code against it.

Scope and known limits, stated rather than implied:

  * The authority for "what is exported" is `go doc -all`, the same rendering
    pkg.go.dev shows. Const and var *blocks* are walked member by member --
    `circuitbreaker.StateOpen` lives inside `const (...)` and a naive scan of
    column-zero declarations misses it.
  * Exported struct fields and interface methods count as surface. They are
    part of the API a consumer compiles against (`health.Check{Name, Probe}`).
  * `contrib/*` is out of scope: those are separate modules versioning
    independently (ADR-0040), so `go list ./...` already excludes them. The
    tool asserts that rather than assuming it.
  * The reverse direction recognises functions, methods and sentinel errors --
    the shapes section 5 writes unambiguously. A deleted *type* is normally
    caught anyway, because its constructor and methods go with it; a *renamed*
    one is caught by the forward direction, since the new name is unlisted.

Output is deliberately ASCII: the Windows console codepage mangles the arrow
and section characters, and a policy tool nobody can read on their own machine
gets ignored.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys

MODULE = "github.com/danielPoloWork/egl-utils-go/v2"
SRC_ROOT = "pkg"
SPEC = os.path.join("docs", "specs", "01_spec_utils.md")
SECTION_START = "## 5. Public Interface"
SECTION_END = "## 6. Verification"

# `go doc` indents code with tabs and prose with spaces; that is what separates
# a const-block member from the sentence documenting it.
CODE_LINE = re.compile(r"^\t+(.*)$")
DECL_FUNC = re.compile(r"^func\s+(?:\([^)]*\)\s+)?([A-Z]\w*)")
DECL_OTHER = re.compile(r"^(?:type|const|var)\s+([A-Z]\w*)")
BLOCK_OPEN = re.compile(r"^(?:const|var)\s+\($")
TYPE_BLOCK_OPEN = re.compile(r"^type\s+([A-Z]\w*)(?:\[[^\]]*\])?\s+(?:struct|interface)\s*\{$")
# A member of a const/var block, a struct field, or an interface method. The
# trailing alternation is what separates `Validate() error` (an interface method,
# and part of the surface) from `Name string` (a struct field, likewise).
MEMBER = re.compile(r"^([A-Z]\w*(?:,\s*[A-Z]\w*)*)(?:\s|\()")

# A section-5 bullet opens with the package it describes: "- workerpool: ..." or
# "- utils (root): ...". The "Error model" and "Versioning surface" bullets start
# with a capital and are correctly skipped.
BULLET = re.compile(r"^- ([a-z][a-z0-9]*)\b[^:]*:(.*)$")

# Declaration shapes for the reverse direction.
CLAIMED_CALL = re.compile(r"\b([A-Z]\w*)(?:\[[^\]]*\])?\(")
CLAIMED_METHOD = re.compile(r"\(\*[A-Za-z]\w*(?:\[[^\]]*\])?\)\.([A-Z]\w*)")
CLAIMED_SENTINEL = re.compile(r"\b(Err[A-Z]\w*)\b")


def die(message: str) -> None:
    print("spec-api lint: %s" % message, file=sys.stderr)
    sys.exit(2)


def go(*args: str) -> str:
    try:
        done = subprocess.run(("go",) + args, capture_output=True, text=True)
    except FileNotFoundError:
        die("the go toolchain is not on PATH; this tool needs it to read the exported surface")
    if done.returncode != 0:
        die("`go %s` failed:\n%s" % (" ".join(args), done.stderr.strip()))
    return done.stdout


def packages() -> list[tuple[str, str]]:
    """(import path, package name) for every package in the root module."""
    out = go("list", "-f", "{{.ImportPath}}\t{{.Name}}", "./...")
    found = []
    for line in out.splitlines():
        if not line.strip():
            continue
        path, name = line.split("\t", 1)
        found.append((path.strip(), name.strip()))
    return found


def exported(pkg: str) -> set[str]:
    """Every exported identifier `go doc -all` reports for one package."""
    idents: set[str] = set()
    in_block = False
    for raw in go("doc", "-all", pkg).splitlines():
        member = CODE_LINE.match(raw)
        if in_block:
            if raw.startswith("}") or raw.startswith(")"):
                in_block = False
            elif member:
                m = MEMBER.match(member.group(1))
                if m:
                    idents.update(part.strip() for part in m.group(1).split(","))
            continue
        if BLOCK_OPEN.match(raw):
            in_block = True
            continue
        opened = TYPE_BLOCK_OPEN.match(raw)
        if opened:
            idents.add(opened.group(1))
            in_block = True
            continue
        for pattern in (DECL_FUNC, DECL_OTHER):
            m = pattern.match(raw)
            if m:
                idents.add(m.group(1))
                break
    return idents


def section5() -> dict[str, str]:
    """Section 5's bullets, keyed by the package each one describes."""
    if not os.path.isfile(SPEC):
        die("%s not found -- run from the repository root" % SPEC)
    text = open(SPEC, encoding="utf-8").read()
    try:
        body = text[text.index(SECTION_START) : text.index(SECTION_END)]
    except ValueError:
        die("could not locate section 5 in %s (headings changed?)" % SPEC)
    listed: dict[str, str] = {}
    for line in body.splitlines():
        m = BULLET.match(line)
        if m:
            listed[m.group(1)] = listed.get(m.group(1), "") + " " + m.group(2)
    return listed


def main() -> int:
    if not os.path.isdir(".git"):
        die("run from the repository root")

    listed = section5()
    problems: list[str] = []
    checked = 0

    for path, name in packages():
        if "/contrib/" in path:
            problems.append(
                "contrib package %s is inside the root module; ADR-0040 keeps contrib/* "
                "in separate modules, outside the versioning surface" % path
            )
            continue
        surface = exported(path)
        checked += len(surface)
        if name not in listed:
            problems.append(
                "package %s exports %d identifier(s) and has no bullet in spec section 5"
                % (name, len(surface))
            )
            continue
        claimed_text = listed[name]
        for ident in sorted(surface):
            if not re.search(r"\b%s\b" % re.escape(ident), claimed_text):
                problems.append(
                    "shipped but unlisted: %s.%s is exported and absent from spec section 5"
                    % (name, ident)
                )
        claims: set[str] = set()
        for pattern in (CLAIMED_CALL, CLAIMED_METHOD, CLAIMED_SENTINEL):
            claims.update(pattern.findall(claimed_text))
        for claim in sorted(claims - surface):
            problems.append(
                "listed but gone: spec section 5 names %s.%s, which the package does not export"
                % (name, claim)
            )

    for name in sorted(set(listed) - {n for _, n in packages()}):
        problems.append(
            "listed but gone: spec section 5 has a bullet for package %s, which does not exist"
            % name
        )

    if problems:
        print("spec-api lint FAILED - %d problem(s):\n" % len(problems))
        for problem in problems:
            print("  * %s" % problem)
        print(
            "\nSection 5 is the enumerated public interface and its closing clause binds SemVer to\n"
            "the exported surface, so the two must agree. Update docs/specs/01_spec_utils.md and\n"
            "add a dated entry to its Amendments block."
        )
        return 1

    print(
        "spec-api lint: OK - %d exported identifiers across %d packages, all named in spec "
        "section 5; no stale entries; contrib/* held outside the versioning surface."
        % (checked, len(listed))
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
