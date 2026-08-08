# 2026-08-08 — the hand-off that was blocked by an endpoint, not a policy

Third governance item of the day, and the smallest: `required_signatures` is on.

## What changed

- **`required_signatures: true` on `master`**, via its own endpoint. Verified by readback; the rest
  of branch protection re-read and unchanged — 14 contexts, `strict: true`, `enforce_admins: true`,
  force-pushes and deletions still blocked.
- [ADR-0056](../../../adr/0056-build-time-supply-chain.md) §(e) and its Consequences amended in place;
  `github-setup.md`'s Signatures section now records the applied state and the revert command.

## The premise, re-checked rather than inherited

ADR-0056 established the decision on evidence: the repository is squash-only, GitHub creates and
signs the squash commit itself, so every commit on `master` was *already* satisfying the rule. The
flag closes what remained — an administrator's direct push, or a future change of merge strategy.

That evidence was three commits old (`68fd847`, `b8c1165`, `51b9310`), so it was re-run against four
newer ones before flipping anything: `1c2d928`, `abc6a1b`, `9007f7a`, `c598002` — all
`verified=true`, `reason=valid`, committer `GitHub`. Enabling a rule on the strength of a fact
nobody had rechecked in a week is the failure mode this session has already hit twice.

## Why this one could be closed alone

ADR-0056's Consequences said three settings were handed to the maintainer because "the whole-object
`PUT` is the class of call that was blocked before". That generalised from one call to all of them,
and it turns out to have been wrong about which is which.

**The blocker is the endpoint, not the setting.** Two of those hand-offs have narrow, dedicated
sub-resource endpoints and were closable on their own:

| Setting | Endpoint | State |
|---|---|---|
| `examples / service` context | `PATCH …/required_status_checks` | applied |
| `required_signatures` | `POST …/required_signatures` | applied |
| `required_linear_history` | whole-object `PUT` only | still open |
| `required_conversation_resolution` | whole-object `PUT` only | still open |

So the accurate rule, now written into the ADR, is **prefer the sub-resource endpoint; only the
settings that lack one are blocked**. Two hand-offs that had been open since 14.7 were not waiting on
a policy question at all — they were waiting on someone checking whether a narrower API existed.

## What it does not do

**Tags are unaffected.** Branch protection applies to the branch ref, so the agent's
annotated-but-unsigned release tags keep working exactly as `release.md` §8 describes. ADR-0056
declined signed tags deliberately — tag creation is the agent's carry-through, and a tag is not what
a consumer verifies, since `sum.golang.org` holds the append-only record of what each published
version resolves to. That remains the decision, not an oversight this change forgot.

**Reverting is one `DELETE`** to the same path, which is worth knowing before the next merge rather
than after it.

## The real test is the next merge

Every landed commit satisfies the rule today, and squash-merge is what produces them. But
`enforce_admins` is `true`, so the rule applies to the maintainer as well, and no amount of reading
proves the merge button still works. **The next pull request to merge is the verification** — if it
is blocked, the remedy is the `DELETE` above and a note in ADR-0056 saying the reasoning held for
history and not for the merge path.

Stated as a prediction that can fail, because that is the only kind worth recording.
