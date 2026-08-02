package env_test

import (
	"fmt"
	"os"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/env"
)

// Every getter here shares one contract, and each example shows both of its
// halves: the variable is set and well-formed, or it is not and the caller's
// fallback is returned. There is no error return — "unset" and "malformed" are
// deliberately the same outcome, so a typo in a deployment's environment
// degrades to the documented default instead of failing a parse far from the
// mistake. Use config.Load where a bad value must be loud.
//
// The examples call os.Setenv only to have something to read; a real program
// reads what its deployment set. The `_ =` is there because os.Setenv returns
// an error the module's linters require handled — it is not part of the usage
// being shown.

func ExampleGetDefault() {
	_ = os.Setenv("EXAMPLE_ADDR", ":9090")
	defer func() { _ = os.Unsetenv("EXAMPLE_ADDR") }()

	fmt.Println(env.GetDefault("EXAMPLE_ADDR", ":8080"))

	// An empty value counts as unset, so `EXAMPLE_ADDR=` in a deployment script
	// does not silently configure the service with an empty address.
	_ = os.Setenv("EXAMPLE_ADDR", "")
	fmt.Println(env.GetDefault("EXAMPLE_ADDR", ":8080"))

	fmt.Println(env.GetDefault("EXAMPLE_NEVER_SET", ":8080"))
	// Output:
	// :9090
	// :8080
	// :8080
}

func ExampleGetInt() {
	_ = os.Setenv("EXAMPLE_MAX_CONNS", "50")
	defer func() { _ = os.Unsetenv("EXAMPLE_MAX_CONNS") }()

	fmt.Println(env.GetInt("EXAMPLE_MAX_CONNS", 10))

	// "fifty" does not parse, so the fallback applies — the same result as if
	// the variable had never been set.
	_ = os.Setenv("EXAMPLE_MAX_CONNS", "fifty")
	fmt.Println(env.GetInt("EXAMPLE_MAX_CONNS", 10))
	// Output:
	// 50
	// 10
}

func ExampleGetBool() {
	_ = os.Setenv("EXAMPLE_DEBUG", "true")
	defer func() { _ = os.Unsetenv("EXAMPLE_DEBUG") }()

	fmt.Println(env.GetBool("EXAMPLE_DEBUG", false))

	// strconv.ParseBool decides what counts: 1/t/T/TRUE/true/True and their
	// false counterparts. "yes" is not among them, so it reads as unconfigured.
	_ = os.Setenv("EXAMPLE_DEBUG", "yes")
	fmt.Println(env.GetBool("EXAMPLE_DEBUG", false))
	// Output:
	// true
	// false
}

func ExampleGetDuration() {
	_ = os.Setenv("EXAMPLE_TIMEOUT", "1500ms")
	defer func() { _ = os.Unsetenv("EXAMPLE_TIMEOUT") }()

	fmt.Println(env.GetDuration("EXAMPLE_TIMEOUT", 5*time.Second))

	// A bare number is not a duration: time.ParseDuration requires a unit, so
	// "30" falls back while "30s" would not. This is the most common way to get
	// the fallback by accident.
	_ = os.Setenv("EXAMPLE_TIMEOUT", "30")
	fmt.Println(env.GetDuration("EXAMPLE_TIMEOUT", 5*time.Second))
	// Output:
	// 1.5s
	// 5s
}
