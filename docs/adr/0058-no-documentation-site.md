# ADR-0058: No documentation site — `pkg.go.dev` is the doc site, and a second one would break the links that matter

- **Status:** Accepted
- **Date:** 2026-08-08
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** ROADMAP 14.11; `docs/workflow/github-setup.md` §4 (which documented the Pages command
  and never ran it); [ADR-0053](0053-runnable-examples-convention.md) and
  [ADR-0054](0054-examples-service-module.md) (the documentation that *does* reach a consumer);
  [ADR-0057](0057-additive-capability-ledger.md) §B (where this deferral is registered)

## Context

`docs/workflow/github-setup.md` §4 has carried a `gh api -X POST .../pages` command since the
repository was generated, under the heading "GitHub Pages from the docs/ folder on the default branch
(optional doc site)". It was never run. `has_pages` is `false`.

That is the third piece of documented-but-unapplied repository configuration this project has found —
after the label set and the merge strategy in the 2026-08-01 audit — and the milestone's instruction
was explicit: **cost it before doing it**, and this item is allowed to end as a recorded decision not
to publish.

So it was costed rather than assumed. Pages from `docs/` would publish **only** `docs/`, and the
measurement is what decided this:

| Measured | Count |
|---|---|
| Markdown files under `docs/` | 160 |
| Relative links that stay inside `docs/` (would work) | 549 |
| **Relative links that escape `docs/` (would 404 on the site)** | **30**, across 12 files |
| Files using ```` ```mermaid ```` (GitHub renders, Jekyll does not) | 0 |
| Directories under `docs/` with no `README.md` or `index.md` | 4 |

Thirty broken links out of 579 is 5%, which on its own would be a fixable annoyance. **What they
point at is the problem.** The escaping links are not incidental: seven go to `AGENTS.md`, three to
`ROADMAP.md`, two to `README.md`, and one each to `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md`. Every
one of those files lives at the repository root and **cannot be published from `docs/`**.

## Decision

**No documentation site. `docs/` stays a directory read through GitHub's own renderer, and
`pkg.go.dev` remains the only published documentation surface this project has.**

Three arguments, in the order they carry weight.

### 1 · The audience that wants a doc site already has a better one

For a Go library the doc site is `pkg.go.dev`. It renders the package documentation, the type
signatures, and — since 14.2–14.4 — **55 runnable examples across all 21 packages**, executed by
`go test` and therefore verified rather than asserted. It is where a consumer arrives, it is indexed,
and it costs nothing to maintain.

What Pages would add is `docs/`: ADRs, journals, benchmark reports, the spec, the compliance
register. That material has a real audience, but it is **contributors**, and a contributor is already
in the repository, where GitHub's file browser gives them a directory tree, blame, history and
search. A site would give them the same text with less navigation.

### 2 · A doc site whose links to the contribution contract 404 is worse than no site

This is the decisive one and it is not a matter of taste. `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`,
`AGENTS.md`, `README.md` and `ROADMAP.md` are root files by convention — GitHub surfaces them from
the root, and 14.9 put two of them there deliberately. Pages from `docs/` cannot serve them.

So the governance documents, which are exactly what someone browsing ADRs is most likely to click
through to, would be the site's dead links. Fixing that means rewriting those 30 links to absolute
`github.com` URLs, which means the site's answer to "how do I contribute?" is to leave the site.

Publishing the whole repository root instead was not considered a serious option: it would serve
`pkg/`, `tools/` and `.github/` as a website.

### 3 · It is a deploy surface bought with nothing

A site needs a build, and under [ADR-0056](0056-build-time-supply-chain.md) a build needs a workflow
whose every action is digest-pinned with a reviewable version comment, a `permissions:` block, and a
place in the supply-chain record. That is a real, recurring cost — one more thing that can go red,
one more entry for Dependabot — incurred to publish text that already renders correctly one click
away.

The four `docs/` subdirectories with no index (`changelog`, `development`, `specs`, `workflow`) and
the absence of any navigation are secondary, but they point the same way: what would ship is 160
files and no structure over them. A doc site is a product. This project has not designed one, and
enabling Pages is not the same as having built it.

## Alternatives Considered

- **Publish, after rewriting the 30 escaping links to absolute URLs.** The honest version of "just
  turn it on". Rejected: it repairs the symptom and leaves argument 1 and argument 3 untouched, and
  it makes 30 links in the repository worse — an absolute `github.com/.../blob/master/AGENTS.md` link
  does not follow a branch, so it reads the wrong file from every feature branch and every fork.
- **Publish a curated subset** — the ADR index and the specs only, with a hand-written index page.
  Genuinely the best version of a doc site here, and rejected as out of scope rather than wrong: it
  is a documentation product with its own information architecture, not a configuration change, and
  14.11 is a metadata item. It is registered in [ADR-0057](0057-additive-capability-ledger.md) §B
  with the trigger below rather than dismissed.
- **A generated site** (Hugo, MkDocs, Docusaurus). Rejected on ADR-0004's instinct applied to
  tooling: a Node or Python site generator is a build-time dependency tree, a lockfile and an upgrade
  cadence, to render Markdown that GitHub already renders.
- **Leave it undecided and revisit if asked.** Rejected because it is the status quo that produced
  this item: a documented command nobody ran and nobody had decided against, which reads as an
  oversight and was one. A recorded "no" is a different artifact from an absent "yes".

## Consequences

- **`has_pages` stays `false`, and `github-setup.md` §4 stops presenting Pages as pending setup.**
  The command is replaced by this decision and a pointer here, so the next person reading the setup
  guide finds an answer instead of an unapplied step.
- **`pkg.go.dev` is now named as the doc site in repository metadata** — 14.11 sets the repository
  homepage to `https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2`, which makes this
  decision visible from the repository's front page rather than only in this file.
- **The 30 escaping links stay as they are**, because they are correct: a relative link from
  `docs/journal/…` to `CONTRIBUTING.md` resolves properly in GitHub's renderer, in a local editor,
  and in a clone. They are only wrong on a site that does not exist.
- **No cost is incurred and none is avoided that was being paid.** Nothing is deleted; `docs/` is
  unchanged.
- **Deferred, additive:** a curated documentation site over a subset of `docs/`, with a hand-written
  index and navigation. Registered in [ADR-0057](0057-additive-capability-ledger.md) §B. **Trigger:**
  a reader who cannot find what they need through the repository — reported, not assumed — or the
  `docs/` corpus growing past the point where GitHub's file browser is a usable index. Neither has
  happened; 160 files across 12 directories with a `README.md` index in most of them is navigable.

## References

- ROADMAP 14.11 — repository metadata, the doc site, and the `v0.1.0` Release.
- `docs/workflow/github-setup.md` §4 — the Pages command this ADR retires.
- [ADR-0053](0053-runnable-examples-convention.md), [ADR-0054](0054-examples-service-module.md) — the
  documentation that reaches a consumer, and why it renders from the tagged tree.
- [ADR-0056](0056-build-time-supply-chain.md) — what any new publishing workflow would have to carry.
- [ADR-0057](0057-additive-capability-ledger.md) §B — where the deferred curated site is registered.
