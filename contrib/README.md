# contrib — driver-backed health probes

Each directory here is a **separate Go module** with its own `go.mod`, its own
dependencies, and its own release tags. They supply
[`health.Check`](../health) probes for dependencies whose clients the core module
is not allowed to import.

| Module | Probes | Driver |
|---|---|---|
| [`contrib/redishealth`](redishealth) | Redis reachability (`PING`) | [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) |
| [`contrib/pgxhealth`](pgxhealth) | PostgreSQL reachability (pool `Ping`) | [`github.com/jackc/pgx/v5`](https://github.com/jackc/pgx) |

## Why these are separate modules

Spec v2 item 22 and [ADR-0003](../docs/adr/0003-adopt-idiomatic-go-root-layout.md)
require that the core module never import a database driver or a cache client. A
consumer who uses `config` and `retry` should not inherit go-redis's or pgx's
dependency tree, their release cadence, or their advisories.

A separate `go.mod` is what delivers that, and it is the *only* thing that does:
`go build ./...` and `go list ./...` in the repository root do not descend into a
nested module, so the core's dependency graph is provably unaffected — verified,
and asserted on every CI run by
[`tools/import_graph_lint.py`](../tools/import_graph_lint.py), which fails if a
`contrib/*` directory ever loses its `go.mod` (those files would silently join the
root module and drag the driver in with them).

The full reasoning, including what was rejected, is in
[ADR-0040](../docs/adr/0040-contrib-submodules.md).

## Installing

Each module is fetched and versioned independently of the core:

```bash
go get github.com/danielPoloWork/egl-utils-go/contrib/redishealth
go get github.com/danielPoloWork/egl-utils-go/contrib/pgxhealth
```

They tag as `contrib/redishealth/vX.Y.Z` and `contrib/pgxhealth/vX.Y.Z` — Go's
convention for nested modules — so a fix to one never forces a release of the core
or of its sibling.

## Using

```go
import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/danielPoloWork/egl-utils-go/health"
	"github.com/danielPoloWork/egl-utils-go/contrib/pgxhealth"
	"github.com/danielPoloWork/egl-utils-go/contrib/redishealth"
)

http.Handle("/readyz", health.Handler(
	redishealth.Check("redis", rdb, redishealth.WithTimeout(2*time.Second)),
	pgxhealth.Check("postgres", pool, pgxhealth.WithTimeout(2*time.Second)),
))
```

`WithTimeout` is worth setting on both. `health.Handler` runs every probe on the
request's context, so without a per-probe bound one unreachable dependency can
hold the readiness endpoint open for as long as the driver's own timeouts allow —
and a readiness check that hangs is worse than one that fails.
[ADR-0026](../docs/adr/0026-health-handler-design.md) deliberately left this to the
probe rather than putting a timeout field on `health.Check`.

## What a probe reports

A probe returns an error naming its package and wrapping the driver's error, so
`errors.Is` and `errors.As` still reach the cause. That error goes to the
consumer's logs, never to the HTTP response: `health.Handler` writes a status-only
body (`{"status":…,"checks":{name:"ok"|"fail"}}`) so an unauthenticated `/readyz`
cannot leak a DSN, a hostname, or a backend version (ADR-0026). Both modules test
that the driver's error text does not appear in the response.

## Adding another contrib module

1. `contrib/<name>/go.mod` declaring
   `github.com/danielPoloWork/egl-utils-go/contrib/<name>` — the path must match
   the directory, which `import_graph_lint.py` checks.
2. Require the core at a released version. Do not add a `replace` or a `go.work`:
   the submodules build against the published core so they are tested the way a
   consumer gets them, and the root module's tooling stays in ordinary
   (non-workspace) mode.
3. Add a `contrib` matrix entry in `.github/workflows/ci.yml` and a `gomod` entry
   in `.github/dependabot.yml` — neither is discovered automatically.
4. The shared [`contrib/.golangci.yml`](.golangci.yml) applies; it deliberately
   omits depguard, since the core's import rules forbid exactly what a contrib
   module exists to do.
