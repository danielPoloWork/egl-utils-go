# 2026-08-08 — the last job that blocked nothing

Milestone 14 is closed and the project is between milestones. This is governance follow-through: the
one hand-off from 14.5 that had already cost something.

## What changed

- **`master` now requires 14 status contexts instead of 13.** `examples / service` is the new one,
  pinned to GitHub Actions (`app_id 15368`) like the other thirteen. `strict: true` unchanged;
  nothing else in branch protection moved.
- **`github-setup.md` gains §3.1** — how to add one context without re-`PUT`ting the whole
  protection object, and when *not* to require a job.
- Dated amendments where the old state was asserted: [ADR-0054](../../../adr/0054-examples-service-module.md)
  (the hand-off itself), [ADR-0055](../../../adr/0055-contrib-release-workflow.md) and
  [ADR-0056](../../../adr/0056-build-time-supply-chain.md) (both said "thirteen"), and
  [BUG-0002](../../../bugs/2026/08/BUG-0002-unbuffered-started-channel-deadlocks-two-examples-service-tests.md).

## The hand-off, and what leaving it open cost

ADR-0054 wrote the warning when the job was created:

> *`examples / service` is not yet a required status check. Adding a job does not add it to branch
> protection, so until the maintainer adds the context, a pull request can merge while this job is
> red.*

That was not theoretical, and the bill arrived four days later. BUG-0002 made the job hang for its
full 10-minute timeout, and **both affected pull requests merged with it red** — #97, where the
failure went unremarked entirely, and #98, which was the **`v2.0.1` release PR**.

A job that fails and blocks nothing is worse than no job: it produces a red mark that everyone learns
to scroll past. The first failure taught exactly that lesson, and the second one arrived before
anybody had unlearned it.

## Order matters: fix first, require second

The context was added **after** BUG-0002 was fixed, and that sequence is the decision rather than an
accident of timing. Requiring a job that fails one run in six does not make the repository stricter —
it makes it unmergeable, and the pressure that follows is to remove the requirement, which lands
further back than where it started.

The related failure mode is worth stating in the same breath: a required context that *never* reports
does not fail a pull request either — it leaves it waiting forever. So before requiring the job I
confirmed it has no `if:` condition and `ci.yml` no path filter, which is what makes
`examples / service` report on every pull request.

## The API detail that belongs in a runbook

The whole-object `PUT .../branches/master/protection` replaces every setting on the branch: anything
missing from the payload is silently switched off. To add one context, patch the sub-resource
instead:

```bash
gh api -X PATCH repos/$OWNER/$REPO/branches/master/protection/required_status_checks --input -
```

Two details that are easy to get wrong and are now written down. Send **`checks`**, not `contexts` —
`contexts` adds the entry *unpinned*, so any app reporting that name would satisfy it, whereas
`checks` carries `app_id`. And the array is **replaced, not merged**, so it has to include the
thirteen that were already there; reading the current state first is both the input and the backup.

The context string must also match the job's rendered name exactly — `examples / ${{ matrix.module }}`
reports as `examples / service`.

## The trap survives, one level down

The `examples` job is a matrix over `module`. A second entry produces `examples / <name>`, which is a
**new context and equally not required by default**. Adding a module to the matrix therefore
reintroduces the exact gap this closes, which is why `examples/README.md`'s "adding a module" checklist
now points at §3.1 and says what happened here rather than just saying "ask the maintainer".

## Verified, not assumed

Read back from a fresh `GET` rather than trusting the `PATCH` response: 14 contexts, 14 checks,
`strict=true`, `examples / service` present with `app_id 15368`, and **zero unpinned contexts**. The
rest of branch protection re-read and unchanged — `enforce_admins` still true, force-pushes and
deletions still blocked.

## Still open

Three hand-offs remain, all outside the repository and all pre-existing: `required_linear_history`,
`required_conversation_resolution`, and `required_signatures` are off; branch protection also requires
no approving review, which is the shape a solo-maintained repository has rather than an oversight.
