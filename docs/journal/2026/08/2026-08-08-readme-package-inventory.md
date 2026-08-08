# 2026-08-08 — the front door that never said what was inside

The README described the module, explained how it is governed, listed the milestones, and never
once told a reader **which packages it contains**.

## What shipped

- **A `Packages` section in `README.md`** — all 21 feature packages in eight functional groups, each
  with a sentence on what it does and a link to its pkg.go.dev page.
- **`consistency_lint.py` check 12, `readme-packages`** — a bijection between the packages listed
  and the directories that exist.

## Why the README and not somewhere new

ADR-0058 decided a week's worth of argument ago that **`pkg.go.dev` is this project's doc site** —
it renders every exported identifier and the 55 runnable examples, and it costs nothing to maintain.
14.11 then set the repository homepage to it for the same reason.

What was missing was the *index into* it. A reader landing on the repository had prose about what the
module is for and no way to see what it ships without browsing `pkg/`. So the section is deliberately
thin: one sentence per package, and the link does the rest. Anything more would be a second copy of
the documentation, which is a drift source rather than a service.

The descriptions were derived from each package's **own doc comment** rather than written from
memory — extracted mechanically, then shortened. Two facts fell out of doing it that way and are
worth keeping: **all 21 packages have a package doc comment** (none missing), and the example count
across the tree is **exactly 55**, which is the number the project has been claiming in the changelog,
the release notes and three ADRs. That is the first time it has been independently counted rather
than carried forward.

The section also states the property that makes a package list *useful* rather than decorative:
**nothing here imports anything else here** ([ADR-0035](../../../adr/0035-import-graph-enforcement.md)),
so taking one package pulls in no siblings. Without that, a list of 21 names reads as 21 things you
have to adopt together.

## The gate, and the deliberate violation that failed to fail

A hand-written table of 21 rows is exactly the artifact that goes stale when the twenty-second
package arrives — and nothing else in the tree forces anyone to touch it. That is this repository's
most-repeated lesson, so the section is gated: `readme-packages` asserts the listed set and the
on-disk set are equal, in both directions, the same discipline the ADR index and bug ledger use.

Verified the way 14.7 taught — by printing what the check *sees* (21 and 21, neither empty, since an
empty set satisfies every assertion) and then by deliberate violation.

**And the second violation did not fail.** Sabotaging a link to `…/pkg/semaphoreX` was supposed to
report a package that does not exist; the run came back clean. The name pattern was anchored to
`[a-z0-9_]+`, so it matched the leading lowercase run — `semaphore` — and stopped at the `X`. The
extracted name was a real package, so the check saw nothing wrong.

The test was badly chosen, and the check was genuinely weaker than intended: **any mistyped link
containing an uppercase character would have passed**. Widening the class to `[A-Za-z0-9_-]+` fixes
it, and both directions now fire on both the uppercase and the lowercase typo.

It is worth being precise about what happened, because the failure mode is subtle: the deliberate
violation *did* do its job. A test that fails to fail is information — it is the only signal that
separates "the check works" from "the check agrees with me today".

## What the gate cannot see, stated rather than discovered

It compares the **set** of packages, never the prose beside them. A row whose description has gone
stale passes.

Checking the text against each package's doc comment was considered and rejected: the README
deliberately paraphrases — shorter, and carrying cross-references (`ADR-0038`, `ADR-0050`) that a doc
comment should not — so an equality check would fail on every honest edit, and a similarity threshold
is a number nobody can justify. The set is the part that can be checked mechanically; the prose is a
review property.

## Where this leaves the project

`master` was at `a824b73` with nothing open when this started. Milestone 15 remains deliberately
unchosen: 49 ledger entries, one fired trigger.
