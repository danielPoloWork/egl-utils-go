// Package env reads environment variables with safe typed fallbacks.
//
// Every getter follows the same contract: it returns the parsed environment
// value when the variable is set to a non-empty, well-formed string, and the
// caller's fallback otherwise — an unset variable, an empty value, or a value
// that does not parse as the target type all yield the fallback. The getters
// never return an error; a malformed value is treated as "not configured", the
// safe default this package is for. Use config.Load for whole structs and
// validation; use these for individual, optional settings.
package env

import (
	"os"
	"strconv"
	"time"
)

// GetDefault returns the value of the environment variable key, or fallback if
// it is unset or empty.
func GetDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// GetInt returns the environment variable key parsed as a base-10 int, or
// fallback if it is unset, empty, or not a valid integer.
func GetInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// GetBool returns the environment variable key parsed as a bool (per
// strconv.ParseBool: 1/t/T/TRUE/true/... and their false counterparts), or
// fallback if it is unset, empty, or not a valid bool.
func GetBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// GetDuration returns the environment variable key parsed as a time.Duration
// (per time.ParseDuration, e.g. "300ms", "1.5h", "2h45m"), or fallback if it is
// unset, empty, or not a valid duration.
func GetDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
