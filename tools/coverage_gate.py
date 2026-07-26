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

MODULE = "github.com/danielPoloWork/egl-utils-go"

# `ok  <pkg>  0.123s  coverage: 97.7% of statements`
COVERAGE_RE = re.compile(r"^(?:ok|---\s*FAIL)\s+(\S+)\s+.*?coverage:\s+([0-9.]+)%")
NO_STATEMENTS_RE = re.compile(r"^ok\s+(\S+)\s+.*coverage:\s+\[no statements\]")


def module_dirs() -> list[str]:
    """The root module plus every contrib submodule.

    `go test ./...` does not descend into a nested module, so without this the
    gate would silently stop covering contrib/* the moment those modules were
    added — a coverage gate that quietly ignores new code is the failure mode this
    tool exists to prevent (ADR-0036, ADR-0040).
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
        return pkg[len(MODULE) :].lstrip("/") or "(root)"

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
