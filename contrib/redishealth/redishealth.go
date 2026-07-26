// Package redishealth supplies a health.Check that probes a Redis server.
//
// It exists as a separate module so the core egl-utils-go module never imports a
// Redis client: a consumer who does not use Redis inherits none of go-redis's
// dependency tree, and this module versions and releases independently of the
// core (ADR-0003, ADR-0040).
//
//	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	http.Handle("/readyz", health.Handler(
//		redishealth.Check("redis", client, redishealth.WithTimeout(2*time.Second)),
//	))
//
// The probe issues a PING and reports the result. health.Handler never writes a
// probe's error to the HTTP response (ADR-0026), so the error returned here is
// for the consumer's logs: it names this package and wraps the driver's error so
// errors.Is and errors.As still reach it.
package redishealth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/danielPoloWork/egl-utils-go/health"
)

// pinger is the only behaviour this package needs from a Redis client. Declaring
// it here rather than taking redis.UniversalClient internally is what makes the
// probe testable without a live server: the exported constructor keeps the
// driver's own interface for ergonomics, and the logic below is written against
// these two methods.
type pinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

type options struct {
	timeout time.Duration
}

// Option configures Check.
type Option func(*options)

// WithTimeout bounds a single probe, independently of the context health.Handler
// passes in.
//
// Worth setting. Handler runs every probe on the request's context, so without a
// per-probe bound one unreachable dependency can hold the endpoint open for as
// long as the driver's own timeouts allow, and a readiness check that hangs is
// worse than one that fails. ADR-0026 deliberately left this to the probe rather
// than putting a timeout field on health.Check, and this is that timeout.
//
// It panics if d is not positive — a configuration error, caught at wiring
// (ADR-0005 idiom).
func WithTimeout(d time.Duration) Option {
	if d <= 0 {
		panic("redishealth: non-positive timeout")
	}
	return func(o *options) { o.timeout = d }
}

// Check returns a health.Check named name that reports whether client's Redis
// server answers a PING.
//
// client is redis.UniversalClient, so a *redis.Client, *redis.ClusterClient or
// *redis.Ring all satisfy it directly. Check panics on an empty name or a nil
// client: both are wiring errors, and catching them here points at the
// construction site rather than at the first request (ADR-0005 idiom).
//
// The nil check catches an untyped nil. A typed nil — a (*redis.Client)(nil)
// stored in the interface — is not detectable this way and will instead fail on
// the first probe, which is the same behaviour any method call on it would have.
func Check(name string, client redis.UniversalClient, opts ...Option) health.Check {
	if client == nil {
		panic("redishealth: nil client")
	}
	return newCheck(name, client, opts...)
}

// newCheck is Check over the narrow interface, so tests can drive every path
// with a fake client.
func newCheck(name string, p pinger, opts ...Option) health.Check {
	if name == "" {
		panic("redishealth: empty check name")
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
			if err := p.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("redishealth: ping %q: %w", name, err)
			}
			return nil
		},
	}
}
