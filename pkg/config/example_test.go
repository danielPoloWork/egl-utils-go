package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/config"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/validator"
)

// writeConfig puts contents in a fresh temporary directory and returns the
// file's path plus its cleanup. A real program is handed the path by its
// deployment; an example cannot read /etc/app.yaml and still run, so it writes
// the file it is about to load. The extension matters — it is what selects the
// decoder.
func writeConfig(name, contents string) (path string, cleanup func()) {
	dir, err := os.MkdirTemp("", "egl-config-example")
	if err != nil {
		panic(err)
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		panic(err)
	}
	return path, func() { _ = os.RemoveAll(dir) }
}

// Load decodes straight into the caller's own type: no intermediate map, no
// type assertions, and a compile error rather than a runtime one when a field
// is renamed.
func ExampleLoad() {
	type Config struct {
		Addr string `yaml:"addr"`
		DSN  string `yaml:"dsn"`
	}

	// ${VAR} and $VAR are expanded from the environment before parsing, which is
	// how a committed file references a secret without containing one. An unset
	// variable expands to empty — pair it with WithStructValidation or a
	// Validator if empty must fail rather than pass silently.
	_ = os.Setenv("EXAMPLE_DB_PASSWORD", "s3cret")
	defer func() { _ = os.Unsetenv("EXAMPLE_DB_PASSWORD") }()

	path, cleanup := writeConfig("app.yaml", `
addr: ":8080"
dsn: "postgres://app:${EXAMPLE_DB_PASSWORD}@localhost/app"
`)
	defer cleanup()

	cfg, err := config.Load[Config](path)
	fmt.Println(err == nil)
	fmt.Println(cfg.Addr)
	// The reference is gone and the environment's value is in its place. The
	// assembled DSN is deliberately not printed: it is a credential, and it
	// would also be a literal secret sitting in a documentation file.
	fmt.Println(strings.Contains(cfg.DSN, "${"), strings.Contains(cfg.DSN, "s3cret"))
	// Output:
	// true
	// :8080
	// false true
}

// The format follows the extension, so the same struct loads from JSON with no
// second call and no format argument. Tag both shapes when either may be used.
func ExampleLoad_json() {
	type Config struct {
		Addr string `json:"addr" yaml:"addr"`
		Port int    `json:"port" yaml:"port"`
	}

	path, cleanup := writeConfig("app.json", `{"addr": ":8080", "port": 5432}`)
	defer cleanup()

	cfg, err := config.Load[Config](path)
	fmt.Println(err == nil, cfg.Addr, cfg.Port)

	// An unrecognised extension is refused rather than guessed at, and the
	// sentinel is reachable with errors.Is.
	bad, cleanupBad := writeConfig("app.txt", "addr: :8080")
	defer cleanupBad()

	_, err = config.Load[Config](bad)
	fmt.Println(errors.Is(err, config.ErrUnsupportedFormat))
	// Output:
	// true :8080 5432
	// true
}

// WithStructValidation applies the validator package's field tags in the same
// call that loads the file, so a misconfigured deployment fails at startup
// instead of at the first request that touches the bad field.
func ExampleWithStructValidation() {
	type Config struct {
		Addr  string `yaml:"addr"  validate:"required"`
		Admin string `yaml:"admin" validate:"required,email"`
		Port  int    `yaml:"port"  validate:"min=1,max=65535"`
	}

	path, cleanup := writeConfig("app.yaml", `
addr: ":8080"
admin: "not-an-address"
port: 70000
`)
	defer cleanup()

	_, err := config.Load[Config](path)
	fmt.Println(err == nil) // off by default: a file with bad values loads fine

	_, err = config.Load[Config](path, config.WithStructValidation())
	fmt.Println(err != nil)

	// Every violated field is reported, not just the first, and errors.As
	// reaches the individual failures through the wrapping Load adds.
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, fe := range verrs {
			fmt.Println(fe.Field, fe.Tag)
		}
	}
	// Output:
	// true
	// true
	// Admin email
	// Port max
}

// Validator is for the invariants a per-field tag cannot express — the
// relationships between fields. It runs after the tags, so an implementation
// may assume every field is individually well-formed and concern itself only
// with the cross-field question it exists for.
func ExampleValidator() {
	path, cleanup := writeConfig("app.yaml", `
min_conns: 20
max_conns: 5
`)
	defer cleanup()

	_, err := config.Load[pool](path)
	fmt.Println(errors.Is(err, errPoolBounds))
	// Output: true
}

// pool implements config.Validator on a pointer receiver — Load looks for the
// method on *T, so either receiver is found.
type pool struct {
	MinConns int `yaml:"min_conns"`
	MaxConns int `yaml:"max_conns"`
}

var errPoolBounds = errors.New("min_conns exceeds max_conns")

func (p *pool) Validate() error {
	if p.MinConns > p.MaxConns {
		return errPoolBounds
	}
	return nil
}

// WithoutEnvExpansion leaves the file bytes verbatim, for the config that
// legitimately contains a '$' — a password, a currency template, a regexp.
// Without it, "$100" expands to the value of the variable named "100" (empty),
// silently corrupting the value rather than failing.
func ExampleWithoutEnvExpansion() {
	type Config struct {
		Password string `yaml:"password"`
	}

	path, cleanup := writeConfig("app.yaml", `password: "pa$$word"`)
	defer cleanup()

	cfg, _ := config.Load[Config](path, config.WithoutEnvExpansion())
	fmt.Println(cfg.Password == "pa$$word")

	expanded, _ := config.Load[Config](path)
	fmt.Println(expanded.Password == "pa$$word")
	// Output:
	// true
	// false
}
