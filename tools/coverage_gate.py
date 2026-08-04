#!/usr/bin/env python3
"""Enforce the per-package statement-coverage floor (ADR-0036, spec v2 section 7).

The floor is applied **per package**, not to a module-wide average. A single
number over the whole module would let a well-covered package subsidise a
neglected one, and with most packages at 100% a module-wide 85% would pass no
matter how bad any individual package became -- a gate that cannot fail is not a
gate. Per package, the threshold binds the weakest package instead.

Packages with no statements (the root, which only carries the module's doc and
version) are skipped rather than counted as zero.

Run it from the repository root:

    python tools/coverage_gate.py            # threshold from THRESHOLD below
    python tools/coverage_gate.py --report   # print the table and always exit 0

Exit status is 0 when every package meets the floor and 1 otherwise, naming each
package that fell short and by how much.
"""

from __future__ import annotations

import os
import re
import subprocess
import sys

# Spec v2 section 7 raises the floor from the provisional 80% to 85%. See
# docs/adr/0036-coverage-gate.md for why the number is what it is and why it is
# applied per package.
THRESHOLD = 85.0

MODULE = "github.com/danielPoloWork/egl-utils-go/v2"
SRC_ROOT = "pkg"

# `ok  <pkg>  0.123s  coverage: 97.7% of statements`
COVERAGE_RE = re.compile(r"^(?:ok|---\s*FAIL)\s+(\S+)\s+.*?coverage:\s+([0-9.]+)%")
NO_STATEMENTS_RE = re.compile(r"^ok\s+(\S+)\s+.*coverage:\s+\[no statements\]")


def module_dirs() -> list[str]:
    """The root module plus every contrib submodule.

    `go test ./...` does not descend into a nested module, so without this the
    gate would silently stop covering contrib/* the moment those modules were
    added — a coverage gate that quietly ignores new code is the failure mode this
    tool exists to prevent (ADR-0036, ADR-0040).

    **examples/* is deliberately absent, and this is the decision rather than an
    oversight** (ADR-0054). Those modules are documentation: nobody imports them,
    so no consumer's correctness rests on their statements. The number says so
    too — `examples/service` measures 56.2%, because `main()` is 17 of its 48
    statements and not one of them is reachable from a test: it binds a port and
    blocks in `lifecycle.WaitForSignals` on the package's process-wide singleton.
    Its composed logic, in service.go, is at 87.1% and would clear the floor on
    its own. Including the module would therefore force one of two bad outcomes —
    lower the floor for the whole repository, or add a second per-module threshold
    to a tool whose entire argument is that there is one floor, applied per
    package. What examples/* gets instead is a CI job that builds, vets and
    *runs* it (`go test -race`), which is the bar that matters for a demo:
    a directory of Go files that only compiles proves nothing.
    """
    dirs = ["."]
    contrib = "contrib"
    if os.path.isdir(contrib):
        for name in sorted(os.listdir(contrib)):
            d = os.path.join(contrib, name)
            if os.path.isfile(os.path.join(d, "go.mod")):
                dirs.append(d)
    return dirs


def main() -> int:
    report_only = "--report" in sys.argv[1:]

    measured: list[tuple[str, float]] = []
    skipped: list[str] = []
    for d in module_dirs():
        proc = subprocess.run(
            ["go", "test", "./...", "-count=1", "-cover"],
            capture_output=True,
            text=True,
            check=False,
            cwd=d,
        )
        if proc.returncode != 0:
            print("module %s:" % d)
            print(proc.stdout)
            print(proc.stderr, file=sys.stderr)
            return proc.returncode

        for line in proc.stdout.splitlines():
            if NO_STATEMENTS_RE.match(line):
                skipped.append(NO_STATEMENTS_RE.match(line).group(1))
                continue
            m = COVERAGE_RE.match(line)
            if m:
                measured.append((m.group(1), float(m.group(2))))

    if not measured:
        print("Coverage gate: no coverage data parsed from `go test -cover`.")
        return 1

    def short(pkg: str) -> str:
        rest = pkg[len(MODULE) :].lstrip("/")
        if rest == SRC_ROOT:
            return "(root)"
        if rest.startswith(SRC_ROOT + "/"):
            rest = rest[len(SRC_ROOT) + 1 :]
        return rest or "(root)"

    measured.sort(key=lambda pair: pair[1])
    failures = [(pkg, pct) for pkg, pct in measured if pct < THRESHOLD]

    if report_only or failures:
        print("Per-package statement coverage (floor %.1f%%):\n" % THRESHOLD)
        for pkg, pct in measured:
            mark = "FAIL" if pct < THRESHOLD else "ok  "
            print("  %s %-16s %5.1f%%" % (mark, short(pkg), pct))
        if skipped:
            print("\n  skipped (no statements): %s" % ", ".join(short(p) for p in skipped))
        print("")

    if failures:
        print("Coverage gate: %d package(s) below the %.1f%% floor\n" % (len(failures), THRESHOLD))
        for pkg, pct in failures:
            print(
                "  [!]   %s is at %.1f%%, %.1f points short. Cover the new behaviour, or -- if a "
                "branch is genuinely unreachable from a test -- say so in the PR and adjust the "
                "floor deliberately." % (short(pkg), pct, THRESHOLD - pct)
            )
        print("\nPolicy: docs/adr/0036-coverage-gate.md, AGENTS.md section 10.")
        return 1

    weakest_pkg, weakest_pct = measured[0]
    print(
        "Coverage gate: OK - %d packages measured, all >= %.1f%% "
        "(weakest: %s at %.1f%%)."
        % (len(measured), THRESHOLD, short(weakest_pkg), weakest_pct)
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
