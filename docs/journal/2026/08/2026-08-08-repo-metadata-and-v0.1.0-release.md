# 2026-08-08 — 14.11: the private reporting form nobody could reach

Milestone 14 reaches **11/12**. The item was three pieces of repository metadata, a doc-site decision
and a missing Release — the smallest brief left in the milestone. It found something none of those
were about.

## What shipped

- **Repository description, 12 topics, and a homepage** pointing at `pkg.go.dev`. All three were
  empty for the project's whole life.
- **Private vulnerability reporting: enabled.** It was off, and two policy documents depend on it.
- **[ADR-0058](../../../adr/0058-no-documentation-site.md)** — no documentation site, costed and
  recorded, with a curated site registered as a deferred capability rather than dismissed.
- **A drafted `v0.1.0` Release**, backfilled from `docs/releases/v0.1.0.md`, with `--latest=false`.
- **`github-setup.md` §4 rewritten** into 4.1–4.4, and `release.md` gained a backfill procedure.

## The finding

`SECURITY.md` tells a reporter to use GitHub's private advisory form. Since 14.9,
`CODE_OF_CONDUCT.md` designates **the same form** for conduct reports — chosen over publishing an
email specifically because it was "the only private, authenticated channel this repository offers".
Two contact links in the issue chooser point there too.

`gh api repos/danielPoloWork/egl-utils-go/private-vulnerability-reporting` returned
**`{"enabled": false}`**.

With that setting off, the form is reachable only by users who can already create a draft security
advisory — maintainers and admins. **The entire audience both documents were written for had no
route at all.** A vulnerability reporter and someone reporting harassment would each have followed
the instructions to a page they could not use.

14.9 reasoned about exactly this failure in the abstract and rejected the GitHub `noreply` address
because it "accepts no incoming mail — a contact that looks real and silently discards reports is
worse than a visibly missing one". Then it chose a channel that was, at that moment, equally unable
to receive anything. The reasoning was right and the verification stopped one level too early: it
checked what the address *was*, and never checked whether the *form* was switched on.

The generalisation is one this project has now hit three times, after the missing labels and the
merge strategy in the 2026-08-01 audit: **a document that points at a configuration is making a claim
about something no gate in this repository can see.** Prose is checkable; repository settings are
not, and they are exactly where the drift accumulates.

It is enabled now, verified by readback rather than assumed, and `github-setup.md` §4.4 carries it as
required with the readback command — not as the web-UI footnote it had been since generation.

## "Three unapplied §4 items" — §4 documented none of them

The roadmap line called the description, topics and homepage "three unapplied `github-setup.md` §4
items". §4 was Discussions, Pages and the security policy. It has never mentioned any of the three.

So they were not unapplied steps; they were **undocumented ones**, which is why nobody applied them.
The fix is therefore a new §4.1 as much as it is a `PATCH` call — writing the values without writing
the section would recreate the same gap for the next repository in the series.

The description is the README's own one-liner, deliberately, so the two cannot drift into two
different claims about what this library is.

## Pages, priced before deciding

The brief said to cost it, and allowed the item to end as a recorded decision **not** to publish. The
measurement decided it:

| | |
|---|---|
| Markdown files under `docs/` | 160 |
| Relative links that stay inside `docs/` | 549 |
| **Links that escape `docs/`** | **30, across 12 files** |
| Files using mermaid | 0 |

Five percent broken would be a fixable annoyance. **What they point at is not.** Pages from `docs/`
publishes only `docs/`, and the escaping links go to `AGENTS.md` (seven), `ROADMAP.md` (three),
`README.md` (two), `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md` — every one a root file by convention,
none of them servable from `docs/`.

So the site's dead links would be precisely the governance documents that someone reading an ADR
clicks through to. And "publish, then fix the links" is worse than it sounds: rewriting them to
absolute `blob/master` URLs makes 30 links wrong *in the repository*, because an absolute link does
not follow a branch and would read `master`'s file from every feature branch and every fork.

Two further arguments are recorded so this is not re-litigated on taste. A Go library's doc site is
**`pkg.go.dev`**, which already renders 55 verified runnable examples to the consumer audience, while
`docs/` serves contributors who are already in the repository and have a file tree, blame and search.
And a site is a **deploy surface**: under ADR-0056 it needs a workflow with every action digest-pinned,
a `permissions:` block and a Dependabot entry, to publish text that renders correctly one click away.

The genuinely good version — a curated subset with a hand-written index — is not dismissed. It is
[ADR-0057](../../../adr/0057-additive-capability-ledger.md) §B with a trigger.

## The ledger's first real exercise, one day old

ADR-0058 defers that curated site, so it carries the `Deferred, additive:` marker — and
`consistency_lint`'s new `ledger-coverage` check **required** the §B row before the tree would pass.

The mechanism adopted yesterday did its job on the first ADR written after it, which is the cheapest
possible confirmation that it works. Confirmed the way 14.7 taught rather than by a green run: the
check's sets were re-read, and `0058` appears in both the marked set (now 14) and the cited set (now
37).

## The backfill trap

`v0.1.0` was tagged 2026-07-12 and never got a Release, while four later tags each have one —
ADR-0055 predicted this as "the asymmetry 14.11 has to clean up" and is amended in place now that it
is gone.

The trap is one flag. **`gh release create` marks a new Release as "Latest" by default**, so
backfilling an *older* tag without `--latest=false` moves the Latest badge off `v2.0.0` and onto a
version that predates every feature package — visible to every visitor immediately, and the kind of
mistake that looks like a botched release. Verified after creating: latest is still `v2.0.0`.

It is a **draft**. AGENTS.md §6.1's boundary is unchanged — the agent drafts a Release, the maintainer
publishes it — and the body says it is a backfill and when, because a Release dated four weeks after
its tag is otherwise unexplained. `release.md` now carries the procedure and the flag.

## Where this leaves the project

Milestone 14 is **11/12**. The last item is **14.12** — release `v2.0.1`, which exists so pkg.go.dev
renders the examples from a tagged tree.

Open for the maintainer: **publish the `v0.1.0` draft** with "Set as the latest release" unchecked.
The four branch-protection hand-offs from earlier milestones are unchanged.
