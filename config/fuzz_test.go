package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/config"
	"github.com/stretchr/testify/require"
)

// fuzzInner is a nested struct, so the fuzzer reaches the decoders' recursive
// paths and the validator's dotted-path reporting.
type fuzzInner struct {
	Name string `json:"name" yaml:"name" validate:"max=8"`
}

// fuzzConfig is the decode target. It carries json, yaml and validate tags so a
// single fuzz target exercises both decoders and — when the option is on — the
// tag-validation path added in roadmap 10.6. Every field is comparable, which is
// what lets the zero-value contract below be asserted with a plain equality.
type fuzzConfig struct {
	Addr  string    `json:"addr"  yaml:"addr"  validate:"required"`
	Admin string    `json:"admin" yaml:"admin" validate:"required,email"`
	Port  int       `json:"port"  yaml:"port"  validate:"min=1,max=65535"`
	Mode  string    `json:"mode"  yaml:"mode"  validate:"oneof=dev prod"`
	Ratio float64   `json:"ratio" yaml:"ratio"`
	Debug bool      `json:"debug" yaml:"debug"`
	Inner fuzzInner `json:"inner" yaml:"inner"`
}

// fuzzExtensions covers each dispatch branch in Load plus one unsupported
// extension, so the format switch is fuzzed rather than only the parsers.
var fuzzExtensions = []string{".json", ".yaml", ".yml", ".conf"}

// FuzzConfigLoader fuzzes the whole Load pipeline — read, environment
// expansion, format dispatch, decode, tag validation — required by spec v2 §7.
//
// The properties asserted are the ones a config loader owes its caller:
//
//   - It never panics. Arbitrary bytes are data, not a programming error, so
//     every malformed input must come back as an error.
//   - On any error it returns the **zero** T, never a partially decoded one.
//     Both encoding/json and gopkg.in/yaml.v3 populate the fields they manage to
//     read before the one that breaks, so this is a real hazard and not a
//     theoretical one: without it, a half-configured struct escapes behind an
//     error return.
//
// Load takes a path rather than bytes, so each input is written to a file. The
// paths are allocated once per worker process (f.TempDir, not t.TempDir) and
// overwritten per execution — creating a directory per execution would make file
// setup dominate the run and starve the fuzzer of executions.
//
// Environment expansion is left enabled for half the inputs. It reads the real
// environment via os.Getenv, which is why the assertions are about absence of
// panic and the zero-value contract rather than about decoded output: an unset
// variable expands to empty on every machine, so nothing here is host-dependent.
func FuzzConfigLoader(f *testing.F) {
	// Valid documents in both formats, then the interesting failure shapes:
	// truncated, wrong-typed, deeply nested, alias-bombed, env-referencing.
	f.Add([]byte(`{"addr":":8080","admin":"ops@example.com","port":8080,"mode":"dev"}`), uint8(0), true, false)
	f.Add([]byte("addr: :8080\nadmin: ops@example.com\nport: 8080\nmode: prod\n"), uint8(1), true, true)
	f.Add([]byte("addr: :8080\ninner:\n  name: short\n"), uint8(2), false, true)
	f.Add([]byte(`{"addr":`), uint8(0), true, false)                            // truncated JSON
	f.Add([]byte(`{"port":"not-an-int","addr":"kept"}`), uint8(0), true, false) // partial-decode hazard
	f.Add([]byte("port: not-an-int\naddr: kept\n"), uint8(1), true, false)      // same, YAML
	f.Add([]byte("addr: ${HOME}\n"), uint8(1), true, false)                     // env expansion
	f.Add([]byte("addr: $\n"), uint8(1), true, false)                           // lone dollar
	f.Add([]byte("a: &x [*x]\n"), uint8(1), true, false)                        // self-referential alias
	f.Add([]byte("\x00\xff\xfe"), uint8(1), true, false)                        // invalid UTF-8
	f.Add([]byte(`{}`), uint8(3), true, false)                                  // unsupported extension
	f.Add([]byte(nil), uint8(0), true, false)                                   // empty file

	dir := f.TempDir()
	paths := make([]string, len(fuzzExtensions))
	for i, ext := range fuzzExtensions {
		paths[i] = filepath.Join(dir, "cfg"+ext)
	}

	f.Fuzz(func(t *testing.T, data []byte, extSel uint8, expandEnv, structValidate bool) {
		path := paths[int(extSel)%len(paths)]
		require.NoError(t, os.WriteFile(path, data, 0o600))

		var opts []config.Option
		if !expandEnv {
			opts = append(opts, config.WithoutEnvExpansion())
		}
		if structValidate {
			opts = append(opts, config.WithStructValidation())
		}

		cfg, err := config.Load[fuzzConfig](path, opts...)
		if err != nil {
			require.Equal(t, fuzzConfig{}, cfg,
				"Load's documented contract: the zero T accompanies any error, never a "+
					"partially decoded value")
			return
		}
		// A success is only reachable for a well-formed document in a supported
		// format; there is nothing to assert about its content, but the value must
		// be usable — reading it here is what would trip the race detector or a
		// decoder that handed back aliased memory.
		_ = cfg.Addr + cfg.Admin + cfg.Mode + cfg.Inner.Name
	})
}
