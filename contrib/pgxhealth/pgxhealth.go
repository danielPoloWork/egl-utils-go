// Package pgxhealth supplies a health.Check that probes a PostgreSQL database
// through a pgx connection pool.
//
// It exists as a separate module so the core egl-utils-go module never imports a
// database driver: a consumer who does not use PostgreSQL inherits none of pgx's
// dependency tree, and this module versions and releases independently of the
// core (ADR-0003, ADR-0040).
//
//	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
//	// ...
//	http.Handle("/readyz", health.Handler(
//		pgxhealth.Check("postgres", pool, pgxhealth.WithTimeout(2*time.Second)),
//	))
//
// The probe acquires a connection from the pool and round-trips to the server,
// so it reports the pool's ability to serve a query rather than merely that a
// socket exists. health.Handler never writes a probe's error to the HTTP response
// (ADR-0026), so the error returned here is for the consumer's logs: it names
// this package and wraps pgx's error so errors.Is and errors.As still reach it.
package pgxhealth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/health"
)

// pinger is the only behaviour this package needs from a pool. Declaring it here
// rather than taking *pgxpool.Pool internally is what makes the probe testable
// without a live database: a pgxpool.Pool cannot be usefully faked, so the
// exported constructor keeps the concrete type for ergonomics and the logic below
// is written against this one method.
type pinger interface {
	Ping(ctx context.Context) error
}

type options struct {
	timeout time.Duration
}

// Option configures Check.
type Option func(*options)

// WithTimeout bounds a single probe, independently of the context health.Handler
// passes in.
//
// Worth setting, and more so here than for a cache: acquiring a pool connection
// can queue behind other work, so an exhausted pool makes the probe wait rather
// than fail. Handler runs every probe on the request's context, so without a
// per-probe bound one saturated pool can hold the endpoint open, and a readiness
// check that hangs is worse than one that fails. ADR-0026 deliberately left this
// to the probe rather than putting a timeout field on health.Check, and this is
// that timeout.
//
// It panics if d is not positive — a configuration error, caught at wiring
// (ADR-0005 idiom).
func WithTimeout(d time.Duration) Option {
	if d <= 0 {
		panic("pgxhealth: non-positive timeout")
	}
	return func(o *options) { o.timeout = d }
}

// Check returns a health.Check named name that reports whether pool can reach its
// PostgreSQL server.
//
// Check panics on an empty name or a nil pool: both are wiring errors, and
// catching them here points at the construction site rather than at the first
// request (ADR-0005 idiom).
func Check(name string, pool *pgxpool.Pool, opts ...Option) health.Check {
	if pool == nil {
		panic("pgxhealth: nil pool")
	}
	return newCheck(name, pool, opts...)
}

// newCheck is Check over the narrow interface, so tests can drive every path with
// a fake pool.
func newCheck(name string, p pinger, opts ...Option) health.Check {
	if name == "" {
		panic("pgxhealth: empty check name")
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return health.Check{
		Name: name,
		Probe: func(ctx context.Context) error {
			if o.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, o.timeout)
				defer cancel()
			}
			if err := p.Ping(ctx); err != nil {
				return fmt.Errorf("pgxhealth: ping %q: %w", name, err)
			}
			return nil
		},
	}
}
