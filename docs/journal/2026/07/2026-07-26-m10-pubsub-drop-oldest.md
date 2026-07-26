# 2026-07-26 — Milestone 10.12: pubsub.WithDropOldest

## What got done

- **Roadmap 10.12** (branch `feat/pubsub-drop-oldest`): an opt-in slow-subscriber policy — on a full
  buffer, evict the **oldest buffered** message so the subscriber sees the freshest. The default stays
  drop-newest, untouched. **New [ADR-0039](../../../adr/0039-pubsub-drop-oldest.md)**, extending ADR-0006.
- **Why the option exists at all**: drop-newest is right for *event*-like streams, where every message is
  independently meaningful and the earliest pending work should not be discarded. It is wrong for
  *state*-like streams — a gauge, a price tick, a progress percentage — where keeping the oldest buffered
  values means the subscriber catches up through data that is already wrong while the freshest value is
  the one thrown away. The godoc now says which to choose and why, since the option is otherwise easy to
  pick for the wrong reason.
- **Both questions flagged last session turned out to matter, and both are resolved deliberately.**
  - **The receive-then-send race.** Evicting from a Go channel is not one operation: a non-blocking
    receive followed by a non-blocking send, with a subscriber or another publisher free to act in
    between. The policy is therefore **best-effort by construction** — one attempt, and if a concurrent
    publisher refills the buffer in the window the new message is dropped instead. Retrying until success
    would make the policy exact but has *no bound*, which breaks the one thing ADR-0006 actually froze:
    `Publish` never blocks. I also rejected a per-subscription lock around the pair — it would add a lock
    to every subscription's delivery path to remove a race whose only consequence is which message gets
    dropped, when the premise is that messages are already being lost.
  - **Which message the drop handler reports.** Under drop-oldest it is the **evicted** message, not the
    incoming one. Reporting the incoming message would have been the change-nothing option, and it would
    lie: that message *is* delivered, so a consumer counting drops would be counting messages that
    arrived safely while the genuinely lost ones went unreported — and the accounting invariant would
    break. `WithDropHandler`'s godoc now states which message it receives under each policy, because the
    distinction matters to anyone logging the payload.
- **The invariant I was most concerned about survives**: per subscription, every published message is
  either delivered or reported dropped, **exactly once**. It holds because an evicted message was
  buffered but never received, so it is never double-counted as both. That is what lets NFR-03's
  benchmark keep asserting `delivered + dropped == subscribers × publishes`, and a dedicated test asserts
  it for the new policy too.
- **Measured cost, confined to the path where messages are already being lost.** On a saturated,
  undrained subscription (median of 5): drop-newest **74.9 ns/op**, drop-oldest **138.4 ns/op** — about
  63 ns for the extra receive and send. The delivering path is untouched and NFR-03's fan-out throughput
  is unchanged at ~6.4 M delivered/s, which I re-ran to confirm the refactor of the delivery loop into a
  `deliver` method cost nothing.
- **`pubsub` goes from 96.4% to 100% coverage.** The gap was exactly the two option validators' panic
  branches (`WithSubscriberBuffer(-1)`, `WithDropHandler(nil)`), never tested; closed while in the
  package. That also lifts the module's third-lowest coverage figure.
- **One test premise of mine was wrong and is now recorded in the code.** I first asserted "a subscriber
  that keeps up loses nothing" by publishing 1000 messages in a tight loop while a goroutine drained:
  729 were dropped. `Publish` never blocks, so an unthrottled publisher *always* outruns a consumer — the
  subscriber was never keeping up. The test now consumes each message before publishing the next, which
  makes the condition real and holds the buffer at one. The comment explains it so the weaker version is
  not restored later.
- Also documented and tested: the option is a **no-op for a rendezvous subscription**
  (`WithSubscriberBuffer(0)`) — there is nothing buffered to evict, so behaviour is drop-newest whatever
  the option says. And receiving from a subscription channel inside `Publish` is safe because `Publish`
  holds the read lock while channels are only closed under the write lock — the same lifecycle argument
  ADR-0006 used, spelled out because "the publisher reads from the subscriber's channel" looks alarming
  until it is.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...` (pubsub run 5×),
  gofumpt clean, golangci-lint v2 0 issues, govulncheck 0 affecting, `consistency_lint.py`,
  `import_graph_lint.py`, `coverage_gate.py` all OK. Additive only — no signature change, no dependency
  change.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (12 of 13)**: 10.1–10.11 merged (#37, #38, #46–#54); 10.12
drafted on `feat/pubsub-drop-oldest`, awaiting the maintainer to open and merge. M10 releases as v1.1.0.

## How the next session resumes

Wait for the 10.12 PR to merge. Then **10.13 `contrib/` nested submodules** — the milestone's last item
and its most structural: `contrib/redishealth` and `contrib/pgxhealth`, each with **its own `go.mod`** and
independent tags (ADR-0003), supplying `health.Check` probes so the core module never imports a driver.
Points to settle before writing code:

- **The whole point is that the core module's graph stays clean.** 10.8's depguard rules already deny
  `redis/go-redis` and `jackc/pgx` by name in the root module, and `./...` does not descend into a nested
  module — so verify that `tools/import_graph_lint.py` and `coverage_gate.py`, which both shell out to
  `go list ./...`, genuinely ignore `contrib/*`. If they do not, they will either miss the submodules or
  fail on them, and that decision should be explicit rather than accidental.
- **CI needs to build and test the submodules**, which the current workflow does not: every job runs
  `go test ./...` from the root and will silently skip `contrib/*`. Decide whether to add a matrix entry
  per submodule or a loop, and whether the coverage gate applies to them.
- **Versioning:** a nested module tags as `contrib/redishealth/vX.Y.Z`. `consistency_lint.py` enforces
  version lockstep between `version.go`, the README badge, and the newest changelog/release files — check
  it does not now demand lockstep with the submodule tags, which version independently.
- These submodules take on driver dependencies deliberately, so ADR-0004's rings apply to them
  separately; the ADR should say which rings govern a contrib module.

Standard footprint per PR (tests + coverage ≥ 85% per package, CHANGELOG `[Unreleased]`, ROADMAP
checkbox, journal, lint, and the three policy tools). Portable Go under `%TEMP%\go-portable` — in the Bash
tool add it as the *unix* path `/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs
the `/v2` module path; `-race` is CI-only, and `-fuzz` needs the restored `pkg/include` +
`src/runtime/cgo` headers.

**10.13 completes Milestone 10**, after which the release carry-through is v1.1.0: roll `[Unreleased]`
into `docs/changelog/v1/v1.1.0.md`, bump `version.go`, update the README badge, write
`docs/releases/v1.1.0.md`, and let `consistency_lint.py` verify the lockstep — then the maintainer merges
and the agent tags.
