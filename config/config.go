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
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	expandEnv bool
}

// Option configures Load.
type Option func(*options)

// WithoutEnvExpansion disables ${VAR}/$VAR environment expansion, leaving the
// file bytes verbatim — use it when the config legitimately contains '$'.
func WithoutEnvExpansion() Option {
	return func(o *options) { o.expandEnv = false }
}

// Load reads path, expands environment references (unless disabled), decodes it
// into T by file extension, and runs T's Validator if it implements one. The
// zero T is returned alongside any error.
func Load[T any](path string, opts ...Option) (T, error) {
	o := options{expandEnv: true}
	for _, opt := range opts {
		opt(&o)
	}

	var cfg T
	// G304: reading a caller-specified config path is this function's contract;
	// the path is the API's first argument, not attacker-influenced input.
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}
	if o.expandEnv {
		data = []byte(os.Expand(string(data), os.Getenv))
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config: parse %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config: parse %s: %w", path, err)
		}
	default:
		return cfg, fmt.Errorf("%w: %q", ErrUnsupportedFormat, filepath.Ext(path))
	}

	if v, ok := any(&cfg).(Validator); ok {
		if err := v.Validate(); err != nil {
			return cfg, fmt.Errorf("config: validate %s: %w", path, err)
		}
	}
	return cfg, nil
}
