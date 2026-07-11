// Package utils is the root of the egl-utils-go module: production-ready Go
// utilities for concurrency, resilience, HTTP middleware, configuration, and
// observability, delivered as small, orthogonal feature packages that compose
// through standard-library contracts (context.Context, net/http.Handler,
// error) only.
//
// The root package carries module-wide metadata such as Version. Feature
// packages live in their own directories at the module root and are imported
// individually, e.g.
//
//	import "github.com/danielPoloWork/egl-utils-go/workerpool"
//
// The layout is decided in ADR-0003 (docs/adr/0003-adopt-idiomatic-go-root-layout.md).
package utils
