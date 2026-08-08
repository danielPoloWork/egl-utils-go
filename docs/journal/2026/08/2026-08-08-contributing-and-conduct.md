# 2026-08-08 — 14.9: the contact that looks real and delivers nothing

Milestone 14 reaches **9/12**. The item is two community-health files, which is the smallest brief in
the milestone and the only one whose hard part was a single line of text.

## What shipped

- **[`CONTRIBUTING.md`](../../../../CONTRIBUTING.md)** — the human path in: setup with the pinned tool
  versions, the four policy tools and *when* they run, branch/commit/PR conventions, the five ADR
  triggers, and how to propose a capability.
- **[`CODE_OF_CONDUCT.md`](../../../../CODE_OF_CONDUCT.md)** — Contributor Covenant 2.1, with an
  enforcement channel that is designated in writing rather than implied.
- **The issue chooser** gains a private conduct route and a link to `CONTRIBUTING.md`; **`SECURITY.md`**
  records that its form receives conduct reports too.
- **`SECURITY.md`'s supported-versions table**, which had been false since `v1.0.0`, now describes the
  versions that exist.
- **`AGENTS.md` §7** gains the coupling rule that keeps the two contracts from drifting.

## The enforcement contact

The Covenant needs somewhere to send a report. The obvious candidate was sitting in every commit:
`106583643+danielPoloWork@users.noreply.github.com`. It is the maintainer's, it is already public in
the git history, and publishing it in a file costs no exposure that has not already happened.

It is also the one address that cannot be used. `users.noreply.github.com` does not accept incoming
mail — it exists so a commit can carry an author without publishing a mailbox. Filling the Covenant's
`[INSERT CONTACT METHOD]` with it would have produced a code of conduct that passes every checker,
satisfies GitHub's community-standards page, and drops every report it receives.

**A contact that looks real and silently discards reports is worse than a visibly missing one**, and
the asymmetry is the whole argument: a missing contact tells the reporter to find another route, while
a broken one tells them they have already reported. The cost of the failure lands entirely on the
person the document exists to protect, and neither party ever learns it happened.

That left the maintainer's work address or the repository's private security-advisory form. The choice
was theirs and they took the form — no email published, and it is the only private, authenticated
channel this repository has.

The known objection is that it is a security channel doing conduct work, and a reviewer noticing that
should find a decision rather than a mistake. So the designation is written down three times, from
both ends: in the Code of Conduct's enforcement section, in the issue chooser as its own private
route, and in `SECURITY.md` from the receiving side, so a report opening with `Code of Conduct` is
triaged as one instead of as a misfiled vulnerability.

One gap the channel genuinely cannot cover is stated instead of papered over. A report **about the
maintainer** goes to GitHub Support, because an escalation path that runs through the person being
complained about is not an escalation path.

## Writing against the failure, not the table of contents

The risk in a `CONTRIBUTING.md` next to an `AGENTS.md` is that it becomes a worse summary of the
better document. What justifies it is the material `AGENTS.md` has no reason to carry, because an
agent working from a session's context does not hit it.

The clearest instance is `gofumpt`. CI pins `v0.10.0`; `go install …@latest` gives `v0.11.0`; the two
disagree about whether arguments may sit on the opening line of a multi-line call. A contributor who
installs the obvious version and formats before committing gets a red `quality` job on a diff they did
not write. That trap cost a red CI round during 14.5 and is now the one paragraph in the file inside a
blockquote. Two neighbours of the same kind: `gofumpt` ignores module boundaries, so a stray file under
`examples/` or `contrib/` reddens the **core's** job, and the nested modules need their commands run
from their own directories because a root `./...` never reaches them.

The one-item-at-a-time rule is given its mechanism rather than restated as an instruction. A squash
merge replaces the branch's commits with one new commit, so **the merged branch is not an ancestor of
`master`** — the fact the 2026-08-01 audit established the expensive way, when 63 stale branches all
looked unmerged to `git merge-base --is-ancestor`. A branch stacked on a merged one therefore carries
commits that no longer exist upstream, and `strict` branch protection makes the rebase mandatory. Told
as a rule it sounds like ceremony; told as a mechanism it is obviously worth obeying.

## §7 refuses a feature-request template

The brief said a capability proposal must not be an issue that goes stale, and the reason is that an
issue records enthusiasm — it accretes +1s and ages. 14.10's ledger schedules on a **trigger**: the
evidence that would move an entry into a milestone.

So §7 asks four questions that are answerable as evidence rather than as a wish: what you need
(behaviour, not the API you imagined), what you do instead today, why it belongs here rather than in
your own code or a `contrib/` module, and what it would break. The second is the load-bearing one —
the workaround is what demonstrates the gap is real and what it costs.

It routes to Discussions and says plainly that the ledger is roadmap 14.10 and not written yet, rather
than linking a file that does not exist. Twenty-odd deferred capabilities across the ADRs are already
waiting for it.

## Two corrections found while the drawer was open

`SECURITY.md` promised that "until `egl-utils-go` reaches `v1.0.0`, only the latest released minor line
receives security fixes", over a table of `0.x` rows. False since v1.0.0 and actively misleading since
v2.0.0. It now names the latest `v2.x`, and records that `v1.1.1` stays resolvable from the proxy —
Go never withdraws a published version — while receiving no fixes, which is the honest shape of a Go
deprecation.

The README's `AGENTS.md` row described it as "how AI agents (and humans) work in this repo". That
parenthesis was carrying the absent contributor documentation; now that something else does the job,
it is gone.

## No ADR

The milestone's ADR numbers are allocated and **0057 is reserved for 14.10** — writing anything into it
here would collide with the ledger. Nor is one warranted on the merits: this item adopts no pattern,
reverses no decision, and changes no gate. It writes down gates that already existed, which is the
opposite of a decision record.

## Verified rather than assumed

Every internal link was resolved against the tree rather than typed from the name that reads naturally
— ADR-0004's file is `0004-runtime-dependency-policy.md`, not `0004-dependency-policy.md`.

And a claim was deleted after checking it. A first draft told contributors to "keep the SPDX header on
new files, matching the ones already there". **0 of 116 Go files carry one**; only the Python tools
under `tools/` do. The sentence described a convention this project does not have, in the document
whose entire job is to tell a newcomer what the conventions are.

## Where this leaves the project

Milestone 14 is **9/12**. Next is **14.10** — the additive-capability ledger, ADR-0057 — which is the
item that decides what Milestone 15 is, and the one §7 above now points contributors at.
