// Package utils is the root of the egl-utils-go module: production-ready Go
// utilities for concurrency, resilience, HTTP middleware, configuration, and
// observability, delivered as small, orthogonal feature packages that compose
// through standard-library contracts (context.Context, net/http.Handler,
// error) only.
//
// The root package carries module-wide metadata such as Version. Feature
// packages live in their own directories under pkg/ and are imported
// individually, e.g.
//
//	import "github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"
//
// This package itself carries only module-wide metadata — Version, and this
// documentation — so importing the module path alone pulls in nothing else.
//
// The layout is decided in ADR-0045 (docs/adr/0045-pkg-layout-and-v2.md), which
// supersedes ADR-0003.
package utils
