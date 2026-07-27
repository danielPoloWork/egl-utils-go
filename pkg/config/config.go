// Package config loads typed configuration from a JSON or YAML file, with
// optional environment-variable expansion and post-load validation.
//
// Load is generic over the destination type, so a consumer decodes straight
// into its own config struct without an intermediate map:
//
//	type Config struct {
//		Addr string `json:"addr" yaml:"addr"`
//		DB   string `json:"db"   yaml:"db"`
//	}
//	cfg, err := config.Load[Config]("config.yaml")
//
// The file format is chosen by extension (.json, .yaml, .yml). Before parsing,
// ${VAR} and $VAR references in the file are replaced with the corresponding
// environment variable (empty when unset) unless WithoutEnvExpansion is passed
// — this is how secrets and per-environment values stay out of the committed
// file. If the decoded value implements Validator, its Validate method runs
// and its error, if any, fails the load.
//
// # Validation
//
// Two validation mechanisms are available and they compose rather than compete:
//
//   - WithStructValidation runs the validator package's tag-based rules
//     (`validate:"required,email,min,max,oneof"`) over the decoded struct — the
//     declarative, per-field layer.
//   - Validator lets the config type check invariants a tag cannot express —
//     relationships between fields, or anything needing real logic.
//
// When both apply, tags run first and a tag failure returns without calling
// Validate, so a Validate implementation may assume every individual field
// already satisfied its own rules and can concern itself only with the
// cross-field questions. Neither mechanism is enabled implicitly: tags require
// the option, and Validate requires the method.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/validator"
)

// ErrUnsupportedFormat is returned (wrapped) when the file extension is not one
// of .json, .yaml, or .yml.
var ErrUnsupportedFormat = errors.New("config: unsupported file format")

// Validator is implemented by a config type that wants to check its own
// invariants after decoding. Load calls Validate on the decoded value (via a
// pointer, so a pointer-receiver method is found) and returns its error.
type Validator interface {
	Validate() error
}

type options struct {
	expandEnv      bool
	validateStruct bool
}

// Option configures Load.
type Option func(*options)

// WithoutEnvExpansion disables ${VAR}/$VAR environment expansion, leaving the
// file bytes verbatim — use it when the config legitimately contains '$'.
func WithoutEnvExpansion() Option {
	return func(o *options) { o.expandEnv = false }
}

// WithStructValidation validates the decoded value against its `validate` field
// tags via validator.Struct, so a config file can be checked declaratively in
// the same call that loads it:
//
//	type Config struct {
//		Addr  string `yaml:"addr"  validate:"required"`
//		Admin string `yaml:"admin" validate:"required,email"`
//		Port  int    `yaml:"port"  validate:"min=1,max=65535"`
//	}
//	cfg, err := config.Load[Config]("config.yaml", config.WithStructValidation())
//
// Rule failures come back as a wrapped validator.ValidationErrors listing every
// violated field, so errors.As reaches the individual *validator.FieldError
// values. It runs before Validator, and a failure skips Validate entirely.
//
// It is off by default: a struct carrying no `validate` tags would pass
// vacuously, so enabling it silently would imply a guarantee that is not there.
//
// Because tag misuse is a programming error rather than bad data, the underlying
// rules panic on a malformed tag, a rule applied to an incompatible type, or a T
// that is not a struct at all — see the validator package documentation. Those
// surface on the first load, which is where a wiring mistake belongs.
func WithStructValidation() Option {
	return func(o *options) { o.validateStruct = true }
}

// Load reads path, expands environment references (unless disabled), decodes it
// into T by file extension, validates it against its `validate` tags when
// WithStructValidation is passed, and runs T's Validator if it implements one.
//
// The zero T is returned alongside any error — never a partially decoded value.
// This matters: a failed decode leaves both encoding/json and gopkg.in/yaml.v3
// having already populated the fields they read before the one that broke, and
// handing that back would let a half-configured struct escape behind an error
// (a security setting silently left at its zero value, for instance). Returning
// the zero value makes the failure total and unambiguous.
func Load[T any](path string, opts ...Option) (T, error) {
	o := options{expandEnv: true}
	for _, opt := range opts {
		opt(&o)
	}

	// zero is what every error path returns; cfg is only handed back on success.
	var zero, cfg T
	// G304: reading a caller-specified config path is this function's contract;
	// the path is the API's first argument, not attacker-influenced input.
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return zero, fmt.Errorf("config: read %s: %w", path, err)
	}
	if o.expandEnv {
		data = []byte(os.Expand(string(data), os.Getenv))
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return zero, fmt.Errorf("config: parse %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return zero, fmt.Errorf("config: parse %s: %w", path, err)
		}
	default:
		return zero, fmt.Errorf("%w: %q", ErrUnsupportedFormat, filepath.Ext(path))
	}

	// Tags first: they are per-field, so a Validate method that runs after them
	// can assume each field is individually well-formed and check only the
	// cross-field invariants it exists for.
	if o.validateStruct {
		if err := validator.Struct(&cfg); err != nil {
			return zero, fmt.Errorf("config: validate %s: %w", path, err)
		}
	}

	if v, ok := any(&cfg).(Validator); ok {
		if err := v.Validate(); err != nil {
			return zero, fmt.Errorf("config: validate %s: %w", path, err)
		}
	}
	return cfg, nil
}
