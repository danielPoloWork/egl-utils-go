// Package middleware provides composable net/http middleware as standard
// decorators.
//
// Every middleware is a decorator with the signature
// func(http.Handler) http.Handler, so they compose by ordinary function
// application and interoperate with any router or third-party middleware
// that speaks the same shape. Middleware that needs no configuration is that
// function directly (RequestID, Recoverer); middleware that is configured is
// a constructor returning it (Logger, Cors) — the split follows the public
// interface frozen in the spec (§5).
//
// Values carried across the chain live in the request context under
// unexported key types, so they cannot collide with a consumer's own keys;
// each is read back through an exported accessor (e.g. RequestIDFrom). The
// middleware in this package own no goroutines. Design decisions are recorded
// in ADR-0013 (the package foundation) and ADR-0014 (Logger).
package middleware
