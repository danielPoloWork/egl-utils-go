package config_test

import (
	"errors"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/config"
	"github.com/danielPoloWork/egl-utils-go/validator"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// tagged carries `validate` tags only — no Validator method — so it isolates the
// tag path from the interface path.
type tagged struct {
	Addr  string `json:"addr"  yaml:"addr"  validate:"required"`
	Admin string `json:"admin" yaml:"admin" validate:"required,email"`
	Port  int    `json:"port"  yaml:"port"  validate:"min=1,max=65535"`
}

// bothChecks carries tags *and* a Validator, so the ordering contract between
// the two mechanisms is observable. Load decodes into a value it owns, so the
// only way to see whether Validate ran is a counter outside the struct; these
// tests therefore must not run in parallel.
type bothChecks struct {
	Addr    string `yaml:"addr"    validate:"required"`
	Replica string `yaml:"replica"`
}

var validateCalls int

func (b *bothChecks) Validate() error {
	validateCalls++
	if b.Replica == b.Addr {
		return errors.New("replica must differ from addr")
	}
	return nil
}

// resetValidateCalls zeroes the counter and returns a func reporting how many
// times Validate ran since.
func resetValidateCalls(t *testing.T) func() int {
	t.Helper()
	validateCalls = 0
	return func() int { return validateCalls }
}

func TestStructValidationPasses(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.yaml", "addr: :8080\nadmin: ops@example.com\nport: 8080\n")
	cfg, err := config.Load[tagged](path, config.WithStructValidation())
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.Addr)
	require.Equal(t, 8080, cfg.Port)
}

func TestStructValidationReportsEveryViolation(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Addr missing, admin not an email, port above max: three independent rules.
	path := write(t, "c.yaml", "admin: not-an-email\nport: 70000\n")
	_, err := config.Load[tagged](path, config.WithStructValidation())
	require.Error(t, err)

	var verrs validator.ValidationErrors
	require.ErrorAs(t, err, &verrs, "the wrapped ValidationErrors survives Load's wrapping")
	require.Len(t, verrs, 3, "every violated field is reported, not just the first")

	var fieldErr *validator.FieldError
	require.ErrorAs(t, err, &fieldErr, "errors.As reaches an individual FieldError")
	require.Contains(t, err.Error(), path, "the error names the file that failed")
}

func TestStructValidationIsOptIn(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The same file that fails above loads cleanly without the option: tags are
	// inert unless asked for.
	path := write(t, "c.yaml", "admin: not-an-email\nport: 70000\n")
	cfg, err := config.Load[tagged](path)
	require.NoError(t, err, "without the option the tags are not enforced")
	require.Equal(t, "not-an-email", cfg.Admin)
}

func TestStructValidationRunsBeforeValidator(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := resetValidateCalls(t)
	// Addr is empty, so the tag layer fails. Validate must not run: it is
	// entitled to assume the per-field rules already passed.
	path := write(t, "c.yaml", "replica: :9090\n")
	_, err := config.Load[bothChecks](path, config.WithStructValidation())
	require.Error(t, err)

	var verrs validator.ValidationErrors
	require.ErrorAs(t, err, &verrs, "the tag failure is what surfaced")
	require.Zero(t, calls(), "a tag failure short-circuits before Validate")
}

func TestStructValidationThenValidatorBothRun(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := resetValidateCalls(t)
	// Tags pass (addr present) but the cross-field invariant fails — the check no
	// tag can express, which is why both layers exist.
	path := write(t, "c.yaml", "addr: :8080\nreplica: :8080\n")
	_, err := config.Load[bothChecks](path, config.WithStructValidation())
	require.Error(t, err)
	require.Equal(t, 1, calls(), "once the tags pass, Validate gets its turn")
	require.Contains(t, err.Error(), "replica must differ from addr")

	var verrs validator.ValidationErrors
	require.NotErrorAs(t, err, &verrs, "this failure came from Validate, not the tag layer")
}

func TestStructValidationWithBothPassing(t *testing.T) {
	defer goleak.VerifyNone(t)
	path := write(t, "c.yaml", "addr: :8080\nreplica: :9090\n")
	cfg, err := config.Load[bothChecks](path, config.WithStructValidation())
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.Addr)
	require.Equal(t, ":9090", cfg.Replica)
}

func TestStructValidationComposesWithOtherOptions(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Setenv("TEST_ADMIN", "ops@example.com")
	path := write(t, "c.yaml", "addr: :8080\nadmin: ${TEST_ADMIN}\nport: 1\n")

	cfg, err := config.Load[tagged](path, config.WithStructValidation())
	require.NoError(t, err, "expansion happens before validation, so the tag sees the real value")
	require.Equal(t, "ops@example.com", cfg.Admin)

	// With expansion disabled the literal "${TEST_ADMIN}" reaches the email rule.
	_, err = config.Load[tagged](path, config.WithStructValidation(), config.WithoutEnvExpansion())
	require.Error(t, err, "the unexpanded placeholder is not a valid email")
}

func TestStructValidationPanicsOnNonStruct(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Asking for struct validation of a non-struct T is a wiring error, and the
	// validator package's contract is to panic on it (ADR-0023) rather than
	// invent a validation failure. Load does not soften that.
	path := write(t, "c.json", `{"a":"b"}`)
	require.Panics(t, func() {
		_, _ = config.Load[map[string]string](path, config.WithStructValidation())
	})
}
