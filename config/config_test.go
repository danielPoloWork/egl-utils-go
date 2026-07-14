package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/config"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

type appConfig struct {
	Addr string `json:"addr" yaml:"addr"`
	Port int    `json:"port" yaml:"port"`
}

// validated has a pointer-receiver Validator, exercising Load's Validate hook.
type validated struct {
	Name string `json:"name" yaml:"name"`
}

func (v *validated) Validate() error {
	if v.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadJSON(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.json", `{"addr": "localhost", "port": 8080}`)
	cfg, err := config.Load[appConfig](path)
	require.NoError(t, err)
	require.Equal(t, appConfig{Addr: "localhost", Port: 8080}, cfg)
}

func TestLoadYAML(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, ext := range []string{"c.yaml", "c.yml"} {
		path := write(t, ext, "addr: localhost\nport: 9090\n")
		cfg, err := config.Load[appConfig](path)
		require.NoError(t, err)
		require.Equal(t, appConfig{Addr: "localhost", Port: 9090}, cfg)
	}
}

func TestLoadExpandsEnv(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Setenv("TEST_ADDR", "10.0.0.1")
	path := write(t, "c.yaml", "addr: ${TEST_ADDR}\nport: 80\n")
	cfg, err := config.Load[appConfig](path)
	require.NoError(t, err)
	require.Equal(t, "10.0.0.1", cfg.Addr)
}

func TestLoadWithoutEnvExpansion(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Setenv("TEST_ADDR", "10.0.0.1")
	path := write(t, "c.json", `{"addr": "${TEST_ADDR}", "port": 80}`)
	cfg, err := config.Load[appConfig](path, config.WithoutEnvExpansion())
	require.NoError(t, err)
	require.Equal(t, "${TEST_ADDR}", cfg.Addr, "expansion disabled leaves the literal")
}

func TestLoadUnsupportedFormat(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.toml", "addr = 'x'")
	_, err := config.Load[appConfig](path)
	require.ErrorIs(t, err, config.ErrUnsupportedFormat)
}

func TestLoadFileNotFound(t *testing.T) {
	defer goleak.VerifyNone(t)
	_, err := config.Load[appConfig](filepath.Join(t.TempDir(), "missing.json"))
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestLoadParseError(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.json", `{"addr": "x", "port": }`)
	_, err := config.Load[appConfig](path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse")
}

func TestLoadYAMLParseError(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.yaml", "addr: [unclosed\n")
	_, err := config.Load[appConfig](path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse")
}

func TestLoadValidatorPasses(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.yaml", "name: service-a\n")
	cfg, err := config.Load[validated](path)
	require.NoError(t, err)
	require.Equal(t, "service-a", cfg.Name)
}

func TestLoadValidatorFails(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.yaml", "name: \"\"\n")
	_, err := config.Load[validated](path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "validate")
	require.Contains(t, err.Error(), "name is required")
}
