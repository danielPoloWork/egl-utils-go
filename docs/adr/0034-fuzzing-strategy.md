# ADR-0034: fuzzing strategy — contract-shaped invariants, a bounded tag space, and the corpus as a regression suite

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0018 (`config.Load`), ADR-0023 (`validator.Struct` — panics on tag misuse), ADR-0033 (`WithStructValidation`), ADR-0005 (loud-by-default), ADR-0030 (spec v2 reconciliation: 10.7 adopted), spec v2.0 §7 (fuzz targets, committed corpora, 10-minute PR budget), roadmap 10.7

## Context

Spec v2 §7 requires "native Go fuzz targets `FuzzConfigLoader` (JSON/YAML inputs) and
`FuzzValidatorTags`; corpora committed; 10-min PR budget". Both named targets sit on code that parses
untrusted input, so this is the module's first systematic robustness gate rather than a box-ticking
exercise. Three problems have to be solved before either target is worth running.

**What is the invariant?** "Does not crash" is the default fuzzing property, and for `config.Load` it
is right — arbitrary file bytes are data, and data must never produce a panic. But `validator.Struct`
**panics by deliberate design** on tag misuse (an unknown rule, a rule on an incompatible type, a
non-numeric parameter), because a malformed struct tag is a programming error, not a validation
failure (ADR-0023, ADR-0005). A naive `FuzzValidatorTags` therefore finds a "crash" within seconds and
reports the documented contract as a bug. The target needs an invariant that distinguishes *contract*
from *defect*.

**How is a tag injected at all?** Struct tags are compile-time constants, so varying one requires
building a type at run time with `reflect.StructOf`.

**How does an input reach `Load`?** `Load` takes a path, not bytes, so every execution needs a file.

## Decision

`FuzzConfigLoader` asserts that `Load` never panics **and** that every error carries the **zero** `T`;
`FuzzValidatorTags` builds a run-time struct type via `reflect.StructOf` from a **bounded table of tag
fragments** and asserts that every outcome stays inside the documented contract — any panic is a string
prefixed `validator: ` and **never a `runtime.Error`**, and any error is a `ValidationErrors`; the seed
corpora are committed under each package's `testdata/fuzz/` so they run as ordinary regression tests on
every `go test`; and CI spends the §7 budget as **two 5-minute runs**, uploading any reproducer as an
artifact.

## Alternatives Considered

- **Asserting "never panics" for `FuzzValidatorTags`** — the conventional fuzzing property. Rejected:
  it contradicts ADR-0023, which makes tag misuse a panic on purpose, so the target would fail
  immediately on correct behaviour. The `runtime.Error` discriminator is what replaces it: a nil
  dereference, an index-out-of-range, or a slice-bounds violation is a genuine defect in the parser or
  an evaluator, while a `validator: `-prefixed string panic is the package doing its job. This also
  catches the *opposite* defect — a malformed tag **silently accepted** with neither panic nor error —
  which a crash-only property cannot see.
- **Weakening `validator`'s panics to errors so the fuzzer can use the simple property** — rejected
  outright: shaping a package's public contract around the convenience of its fuzz target is backwards,
  and ADR-0033 already declined to soften those panics from `config`'s side.
- **Fuzzing tag text byte by byte** — the obvious reading of "fuzz the tags", and it would explore the
  parser more finely. Rejected for a concrete resource reason: `reflect.StructOf` **caches every type
  it constructs and never evicts**, so unbounded tag text allocates a fresh permanently-cached type per
  execution. At the ~20 k exec/s this target actually achieves, a 5-minute run would construct millions
  of types and exhaust the runner's memory long before the budget expired. Combining ≤ 3 fragments from
  a fixed 16-entry table bounds distinct types to `16³ × 11 ≈ 45 000` — flat memory — while the table
  itself carries every malformed shape that matters (unknown rule, empty parameter, non-numeric
  parameter, an integer beyond `int64`, stray whitespace). The field *value* is fuzzed without
  restriction, because that costs nothing. **This bound is load-bearing: removing it turns the job into
  an out-of-memory kill, not a better fuzzer.** Observed behaviour supports the choice — the corpus
  saturated at ~205 interesting inputs and stopped growing, which is the bounded space being explored
  exhaustively rather than a fuzzer starved of novelty.
- **Extracting a byte-oriented `config.Decode([]byte, ext)` so the fuzzer avoids the filesystem** —
  fuzzing would run far faster (the file write is why this target manages ~500 exec/s against the
  validator's ~20 000). Rejected because it would add public API purely to serve a test, and the file
  read is part of what §7 asks to be fuzzed. Instead the paths are created **once per worker process**
  (`f.TempDir`, not `t.TempDir`) and overwritten per execution; a directory per execution would make
  setup dominate the run outright.
- **Committing the fuzzer's generated corpus** (the several hundred cache entries a run produces) —
  rejected as noise: those files are mutation artifacts, mostly random bytes selected for coverage, and
  they carry no intent a reader can maintain. The committed corpus is instead **hand-authored around
  known-hostile documents** — YAML anchor/alias expansion bombs, self-referential aliases, merge keys,
  200-deep nesting in both formats, duplicate keys, tab indentation, a UTF-8 BOM, invalid UTF-8, NUL
  bytes, integer overflow, unterminated and nested `${...}` expansions — which is what makes it
  valuable as a permanent regression suite. Generated reproducers still get committed **when a run
  finds one**, which is exactly the case where a mutation artifact has earned its place.
- **A single 10-minute `-fuzz` run** — `-fuzz` accepts one target per invocation, so this is not
  available; the budget is split evenly instead.
- **`continue-on-error` on the fuzz job** to keep a flaky gate from blocking PRs — rejected: §7 calls
  it a gate, and a crash found by fuzzing is exactly the class of failure that should stop a merge.

## Consequences

- **A latent contract violation was found and fixed by writing the target.** `Load`'s godoc promised
  "The zero T is returned alongside any error", but it returned the decode target, and both
  `encoding/json` and `gopkg.in/yaml.v3` populate the fields they read *before* the one that fails. A
  malformed file therefore handed back a half-configured struct behind an error — verified concretely:
  `{"addr":"kept","port":"not-an-int"}` yielded `{Addr:"kept", Port:0}`. Every error path now returns
  the zero value. This is a **bug fix, not a breaking change**: the documented behaviour was always
  "zero", and code relying on the partial value was relying on a defect. It is covered by an ordinary
  regression test as well as the fuzz invariant, because a contract this quiet should not depend on the
  fuzzer being run.
- Both targets ran clean locally before merge: `FuzzValidatorTags` 1.65 M executions, `FuzzConfigLoader`
  ~51 k (the file write is the difference). 60 hand-authored corpus entries execute on every `go test`
  in well under a second, so the regression value is paid for on every CI run, not only in the fuzz job.
- The fuzz job adds ~10 minutes of wall clock to CI, in parallel with the other jobs. A reproducer is
  uploaded as an artifact on failure, without which a CI-only crash would be unreproducible.
- **A local-toolchain limitation worth recording:** `-fuzz` rebuilds the instrumented standard library
  and therefore needs the assembler headers under `GOROOT/pkg/include` and `GOROOT/src/runtime/cgo`. A
  pruned portable Go distribution can be complete enough for `build`, `vet`, `test` and `bench` while
  still failing to fuzz. Restoring those directories from the distribution archive fixes it; like
  `-race`, treat CI as the authority.
- Deferred: fuzz targets for the remaining parsers of untrusted input as they appear (there are none
  today beyond these two), and a nightly long-running fuzz job should the 10-minute PR budget prove too
  shallow.

## References

- Spec v2.0 §7 (fuzz targets, committed corpora, 10-minute PR budget).
- ADR-0023 (tag misuse panics — the reason the naive invariant is wrong), ADR-0018 and ADR-0033
  (`Load`'s contract and the validation layering), ADR-0005 (loud-by-default).
- `config/fuzz_test.go`, `validator/fuzz_test.go`, the corpora under
  `config/testdata/fuzz/FuzzConfigLoader/` and `validator/testdata/fuzz/FuzzValidatorTags/`,
  `TestLoadReturnsZeroValueOnError`, and the `fuzz` job in `.github/workflows/ci.yml`.
- `reflect.StructOf` type-cache behaviour; Go fuzzing corpus file format (`go test fuzz v1`, one call
  expression per argument).
