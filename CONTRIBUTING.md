# Contributing to `egl-utils-go`

Thank you for considering a contribution. This document is the path in for a **human**
contributor: what the project accepts, how to set up locally, and which gates your change has
to pass before it is reviewable.

[`AGENTS.md`](AGENTS.md) is the parallel contract for **AI agents** working in this repository.
The two describe the same quality bar and the same git workflow; `AGENTS.md` adds the rules that
only apply to an agent (it drafts pull requests and never merges them). If the two ever disagree
about a gate, `AGENTS.md` is the source of truth and the disagreement is a bug in this file —
please report it.

---

## 1. Start with an issue or a discussion, not a pull request

The single most useful thing you can do is tell us what you intend to change **before** you write
it. This project deliberately keeps a small surface, so the most common reason a finished pull
request is declined is not its quality — it is that the capability was not wanted.

| You have | Use |
|---|---|
| A reproducible defect | A [bug report](.github/ISSUE_TEMPLATE/bug_report.yml) |
| A proposal for new capability | See [§7](#7-proposing-a-capability) — **not** a bare issue |
| A question, or an idea you want to think through | [Discussions](https://github.com/danielPoloWork/egl-utils-go/discussions) |
| A security vulnerability | **Never a public issue.** [`SECURITY.md`](SECURITY.md) |
| A code-of-conduct concern | [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) |

Documentation fixes, typo corrections and test improvements do not need this step. Send them
directly.

## 2. What this project accepts

Two standing constraints decide most proposals before anything else does.

**The dependency policy is strict and it is an architectural decision, not a preference.**
[ADR-0004](docs/adr/0004-runtime-dependency-policy.md) allows the core module exactly three runtime
dependencies. A change that adds a fourth will be declined unless it comes with the argument to
reopen that ADR — which is a legitimate thing to propose, and a large one. Integrations that need
a third-party driver live in [`contrib/`](contrib/) as their own modules with their own tags, which
is what keeps them off every consumer's dependency graph.

**Surface is added on evidence, not on anticipation.** Roughly twenty capabilities sit deferred
across the ADRs, each argued as *additive later, so free to omit now*. That is a promise to wait
for a real consumer, not a backlog. See [§7](#7-proposing-a-capability).

## 3. Local setup

| Tool | Version | Why this one |
|---|---|---|
| Go | **1.25** floor; CI builds on 1.25 **and** 1.26 | `go.mod` sets the language floor; both toolchains must pass |
| Python | 3.12 | runs the four policy tools |
| `gofumpt` | **exactly `v0.10.0`** | see the warning below |
| `govulncheck` | `v1.4.0` | pinned in CI |
| `golangci-lint` | per [`.golangci.yml`](.golangci.yml) | `govet`, `staticcheck`, `errcheck`, `revive`, `gosec` |

```bash
go install mvdan.cc/gofumpt@v0.10.0
go install golang.org/x/vuln/cmd/govulncheck@v1.4.0
```

> **`gofumpt@latest` will fail CI.** CI pins `v0.10.0`, and `v0.11.0` formats differently — it
> accepts arguments on the opening line of a multi-line call where `v0.10.0` wants that line bare.
> Installing the newest version and formatting with it produces a diff the `quality` job rejects.
> Install the pinned version explicitly.

`gofumpt` ignores module boundaries and walks the whole tree, so formatting is a property of the
repository rather than of the module you are editing: a badly formatted file under `examples/` or
`contrib/` turns the core's `quality` job red.

The everyday loop:

```bash
go build ./...
go test ./...
go test -race ./...          # requires cgo and a C toolchain
go vet ./...
gofumpt -l .                 # must print nothing
golangci-lint run
govulncheck ./...
```

Working on a nested module (`contrib/*`, `examples/*`) means running these **from that directory** —
each is its own module and `./...` at the root does not reach it.

## 4. The gates run before the pull request, not in review

**Run all four policy tools locally and pass them before you open anything.** Each one gates CI, so
a failure here is a red build either way; the only thing running them late buys you is a slower
round trip. They are fast — seconds, not minutes.

```bash
python tools/consistency_lint.py     # cross-artifact congruence (version lockstep, ADR index,
                                     # patterns, spec coverage map, milestones, bug ledger,
                                     # posture, action pins, workflow permissions,
                                     # additive-capability ledger coverage)
python tools/import_graph_lint.py    # dependency rings + internal edges (ADR-0035)
python tools/coverage_gate.py        # >= 85% statements, per package (ADR-0036)
python tools/spec_api_lint.py        # spec section 5 <-> exported surface (ADR-0043)
```

Capture their output to a file rather than piping it through `head` or `tail` — a truncated
failure has cost this project a diagnosis before.

Beyond those, CI enforces the bar in [`AGENTS.md` §10](AGENTS.md): the build and test matrix across
Linux, Windows and macOS on both toolchains, `go test -race`, a fuzzing budget, a reproducible
benchmark run, an SBOM that must be byte-identical to the committed one, and the nested-module jobs.
Several of these are required status checks on `master`, so they are not advisory.

**Coverage is a floor of 85% statements applied per package, not to the module average.** One
under-tested package fails the gate no matter how well covered everything else is.

## 5. Branch, commit, pull request

**Branch** — `<type>/<short-kebab-description>`, with `type` drawn from the list in the next
paragraph. Keep the description under about 40 characters and favour the *what*.

**Commits** — [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>(<scope>): <imperative subject, <= 72 chars>

<body — explain WHY, not WHAT; wrap at ~72 columns>

<optional footers: BREAKING CHANGE: ... | Refs: #<issue> | ADR-XXXX>
```

The type set is fixed, and it is also the label set — [`.github/labels.yml`](.github/labels.yml) is
the canonical copy:

`feat` · `fix` · `refactor` · `perf` · `docs` · `test` · `build` · `chore` · `ci` · `security`

**One pull request carries exactly one type**, and the label matches the lead commit's type. If you
do not have permission to set labels, do not worry about it — name the type in your title, which is
where it is taken from, and a maintainer will apply it. (`contrib` is a *scope* label applied
alongside a type label, not a type of its own.)

**Scopes** for this repository: `concurrency`, `resilience`, `middleware`, `config`, `logging`,
`storage`, `validation`, `lifecycle`, `api`, `build`, `tests`, `docs`, `ci`.

**The pull-request body becomes the permanent commit message.** This repository squash-merges with
the merge commit taken from the PR title and body, so what you write in
[the template](.github/PULL_REQUEST_TEMPLATE.md) is what `git log` shows on `master` forever. Fill
it in as a summary someone will read in two years, not as review scaffolding.

**One item per pull request, and do not stack them.** Branch from the current `master`, finish, and
wait for the merge before starting the next thing. The reason is mechanical rather than stylistic: a
squash merge replaces your commits with a single new one, so **the merged branch is not an ancestor
of `master`**. A second branch stacked on the first therefore carries commits that no longer exist
upstream, and every stacked branch becomes a rebase you have to perform under conflict. Branch
protection also has `strict` set, so a pull request must be up to date with `master` before it can
merge — rebase and `git push --force-with-lease` on **your own branch** when master moves. Never
force-push `master`.

## 6. When a change needs an ADR

An [Architecture Decision Record](docs/adr/) is required when:

- the change affects the **public API or compatibility** — including a constant whose *value* is
  the contract, which is why a default cost change was major-only;
- **two reasonable options exist** and the rationale is not obvious from the code;
- a **design pattern** is adopted, refined or deliberately rejected ([§8 of `AGENTS.md`](AGENTS.md));
- it **supersedes or amends** an existing ADR — amend in place with a dated note when the original
  reasoning still stands and only a detail moved;
- it is **security-relevant** under the enterprise posture: authentication, authorisation, crypto,
  data handling, a trust boundary, or a dependency with a known CVE surface. Under this posture that
  is never an undocumented judgement call.

Use [`docs/adr/template.md`](docs/adr/template.md), number sequentially, and add the index row in the
same pull request — `consistency_lint.py` checks that the index and the files agree in both
directions.

**If your ADR defers a capability**, write the deferral as `Deferred, additive: <what>` and add its
row to the [additive-capability ledger](docs/adr/0057-additive-capability-ledger.md) in the same pull
request, with the trigger that would schedule it. `consistency_lint.py` checks that too. A deferral
recorded only in an ADR's Consequences section is invisible: that is how eleven of them came to be
built without the deferring ADR ever being updated.

An ADR records the decision **and the argument that lost**. An ADR that lists only what was chosen
is half a record: the next contributor's real question is why the obvious alternative is not there.

Documentation is part of the deliverable, not a follow-up: every pull request keeps `README.md`,
`ROADMAP.md`, `CHANGELOG.md`, the ADRs and the patterns catalogue in sync with itself.

## 7. Proposing a capability

**Please do not open a feature issue that will sit.** This project's answer to "wouldn't it be nice
if…" is deliberately slow, and an issue is the wrong container for it — it goes stale, it accretes
+1s, and it records enthusiasm rather than evidence.

What moves a capability into a milestone is a **trigger**: a statement of the evidence that would
justify building it. So a good proposal answers four questions.

1. **What do you need?** The behaviour, not the API you imagined for it.
2. **What are you doing instead today?** The workaround is the most useful part of the report — it
   is what shows the gap is real and how much it costs.
3. **Why does it belong in this module** rather than in your own code or in a `contrib/` module?
4. **What would it break?** Surface added is surface supported forever.

Open it as a [Discussion](https://github.com/danielPoloWork/egl-utils-go/discussions). Accepted
proposals land in the **[additive-capability ledger](docs/adr/0057-additive-capability-ledger.md)** —
a table of deferred capabilities, each with the trigger that would schedule it, so the milestone
after the current one is chosen from demand rather than invented.

**Read the ledger first.** It already holds 49 capabilities the project has considered and deferred,
each with the argument for deferring it and the evidence that would change the answer. Three things
you may find there:

- **Your capability is already an entry.** Then the useful contribution is not a proposal but the
  trigger: say that you need it, what you do instead today, and what that costs. An entry moves when
  its trigger fires, and most of them fire on exactly that.
- **It is in §C, already discharged.** Eleven were, and the ADRs that deferred them never said so.
- **It is in §D**, meaning it turned out not to be additive after all and belongs to a future major.

A proposal that is declined for now is not rejected work. It is an entry with a trigger that has not
fired.

## 8. Code of Conduct and licensing

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

This project is **MIT** licensed and there is **no CLA**. Contributing means you agree your work is
licensed under the [LICENSE](LICENSE), and that you have the right to contribute it. New files
follow the conventions already in the tree: Go files open with package documentation and carry a
doc comment on every exported identifier; the Python tools under `tools/` carry an SPDX header.
