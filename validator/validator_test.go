package validator_test

import (
	"fmt"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/validator"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

func TestStructValidReturnsNil(t *testing.T) {
	defer goleak.VerifyNone(t)
	type User struct {
		Name  string `validate:"required,min=2,max=20"`
		Email string `validate:"required,email"`
		Age   int    `validate:"min=0,max=150"`
		Role  string `validate:"oneof=admin user guest"`
	}
	require.NoError(t, validator.Struct(User{Name: "Ada", Email: "ada@example.com", Age: 37, Role: "admin"}))
}

func TestStructAcceptsPointer(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		Name string `validate:"required"`
	}
	require.NoError(t, validator.Struct(&T{Name: "x"}))
	require.Error(t, validator.Struct(&T{}))
}

func TestRequired(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		S string   `validate:"required"`
		N int      `validate:"required"`
		P *int     `validate:"required"`
		L []string `validate:"required"`
	}
	err := validator.Struct(T{})
	require.Error(t, err)
	var ve validator.ValidationErrors
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve, 4, "every zero-valued required field fails")

	n := 1
	require.NoError(t, validator.Struct(T{S: "x", N: 1, P: &n, L: []string{"a"}}))
}

func TestEmail(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		Email string `validate:"email"`
	}
	for _, ok := range []string{"a@b.com", "user.name+tag@sub.example.co"} {
		require.NoError(t, validator.Struct(T{Email: ok}), ok)
	}
	for _, bad := range []string{"", "plainstring", "no@dot", "a b@c.com", "@x.com", "x@"} {
		require.Error(t, validator.Struct(T{Email: bad}), bad)
	}
}

func TestMinMaxString(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		S string `validate:"min=2,max=4"`
	}
	require.NoError(t, validator.Struct(T{S: "ab"}))
	require.NoError(t, validator.Struct(T{S: "abcd"}))
	require.NoError(t, validator.Struct(T{S: "café"}), "length is counted in runes, not bytes")
	require.Error(t, validator.Struct(T{S: "a"}))
	require.Error(t, validator.Struct(T{S: "abcde"}))
}

func TestMinMaxNumber(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		I int     `validate:"min=1,max=10"`
		U uint    `validate:"max=5"`
		F float64 `validate:"min=0.5"`
	}
	require.NoError(t, validator.Struct(T{I: 5, U: 5, F: 0.5}))
	require.Error(t, validator.Struct(T{I: 0, U: 5, F: 1}), "I below min")
	require.Error(t, validator.Struct(T{I: 11, U: 5, F: 1}), "I above max")
	require.Error(t, validator.Struct(T{I: 5, U: 6, F: 1}), "U above max")
	require.Error(t, validator.Struct(T{I: 5, U: 5, F: 0.1}), "F below min")
}

func TestMinMaxCollectionLength(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		L []int          `validate:"min=1,max=3"`
		M map[string]int `validate:"max=2"`
	}
	require.NoError(t, validator.Struct(T{L: []int{1, 2}, M: map[string]int{"a": 1}}))
	require.Error(t, validator.Struct(T{L: nil, M: nil}), "empty slice below min=1")
	require.Error(t, validator.Struct(T{L: []int{1, 2, 3, 4}, M: nil}), "slice above max")
	require.Error(t, validator.Struct(T{L: []int{1}, M: map[string]int{"a": 1, "b": 2, "c": 3}}), "map above max")
}

func TestOneOf(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		Color string `validate:"oneof=red green blue"`
		Level int    `validate:"oneof=1 2 3"`
	}
	require.NoError(t, validator.Struct(T{Color: "green", Level: 2}))
	require.Error(t, validator.Struct(T{Color: "purple", Level: 2}))
	require.Error(t, validator.Struct(T{Color: "red", Level: 9}))
}

func TestOneOfNumericAndBoolKinds(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		U uint    `validate:"oneof=1 2 3"`
		F float64 `validate:"oneof=0.5 1.5"`
		B bool    `validate:"oneof=true"`
	}
	require.NoError(t, validator.Struct(T{U: 2, F: 1.5, B: true}))
	require.Error(t, validator.Struct(T{U: 9, F: 1.5, B: true}), "U not listed")
	require.Error(t, validator.Struct(T{U: 2, F: 9.9, B: true}), "F not listed")
	require.Error(t, validator.Struct(T{U: 2, F: 1.5, B: false}), "B not listed")
}

func TestMinMaxUintFloatBranches(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		U uint    `validate:"min=2"`
		F float64 `validate:"max=10"`
	}
	require.NoError(t, validator.Struct(T{U: 2, F: 10}))
	require.Error(t, validator.Struct(T{U: 1, F: 10}), "U below min")
	require.Error(t, validator.Struct(T{U: 2, F: 11}), "F above max")
}

func TestEmptyRuleSegmentTolerated(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		S string `validate:"required, ,min=1"` // stray empty segment is ignored
	}
	require.NoError(t, validator.Struct(T{S: "x"}))
	require.Error(t, validator.Struct(T{}))
}

func TestNestedStruct(t *testing.T) {
	defer goleak.VerifyNone(t)
	type Address struct {
		Zip string `validate:"required,min=5"`
	}
	type User struct {
		Name    string `validate:"required"`
		Address Address
	}
	err := validator.Struct(User{Name: "Ada", Address: Address{Zip: "12"}})
	require.Error(t, err)
	var fe *validator.FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, "Address.Zip", fe.Field, "nested field paths are dotted")
	require.Equal(t, "min", fe.Tag)
}

func TestNestedPointerStruct(t *testing.T) {
	defer goleak.VerifyNone(t)
	type Address struct {
		Zip string `validate:"required"`
	}
	type User struct {
		Address *Address
	}
	require.NoError(t, validator.Struct(User{Address: nil}), "a nil pointer struct is not descended into")
	err := validator.Struct(User{Address: &Address{}})
	require.Error(t, err)
	var fe *validator.FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, "Address.Zip", fe.Field)
}

func TestAggregatesAllFailures(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		A string `validate:"required"`
		B string `validate:"email"`
		C int    `validate:"min=10"`
	}
	err := validator.Struct(T{B: "bad"})
	var ve validator.ValidationErrors
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve, 3)
	require.Contains(t, err.Error(), "\"A\"")
	require.Contains(t, err.Error(), "\"B\"")
	require.Contains(t, err.Error(), "\"C\"")
}

func TestUnexportedAndUntaggedSkipped(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		Tagged   string `validate:"required"`
		Untagged string // no tag -> never checked
		hidden   string //nolint:unused // unexported -> skipped by validation
	}
	require.NoError(t, validator.Struct(T{Tagged: "x"}))
}

func TestDashTagSkipped(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		Skip string `validate:"-"`
	}
	require.NoError(t, validator.Struct(T{}))
}

func TestPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	type Good struct {
		S string `validate:"required"`
	}
	type EmailOnInt struct {
		N int `validate:"email"`
	}
	type MinOnBool struct {
		B bool `validate:"min=1"`
	}
	type UnknownRule struct {
		S string `validate:"wat"`
	}
	type BadIntParam struct {
		N int `validate:"min=abc"`
	}
	type BadUintParam struct {
		U uint `validate:"min=abc"`
	}
	type BadFloatParam struct {
		F float64 `validate:"max=abc"`
	}
	type OneOfOnSlice struct {
		L []int `validate:"oneof=1 2"`
	}
	cases := []struct {
		name string
		fn   func()
	}{
		{"nil value", func() { _ = validator.Struct(nil) }},
		{"non-struct", func() { _ = validator.Struct(42) }},
		{"nil pointer", func() { _ = validator.Struct((*Good)(nil)) }},
		{"email on int", func() { _ = validator.Struct(EmailOnInt{}) }},
		{"min on bool", func() { _ = validator.Struct(MinOnBool{}) }},
		{"unknown rule", func() { _ = validator.Struct(UnknownRule{S: "x"}) }},
		{"bad int param", func() { _ = validator.Struct(BadIntParam{N: 5}) }},
		{"bad uint param", func() { _ = validator.Struct(BadUintParam{U: 5}) }},
		{"bad float param", func() { _ = validator.Struct(BadFloatParam{F: 5}) }},
		{"oneof on slice", func() { _ = validator.Struct(OneOfOnSlice{L: []int{1}}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Panics(t, tc.fn)
		})
	}
}

// TestMinMaxStringProperty asserts the length bounds hold for arbitrary strings.
func TestMinMaxStringProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	type T struct {
		S string `validate:"min=3,max=8"`
	}
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.String().Draw(rt, "s")
		n := len([]rune(s))
		err := validator.Struct(T{S: s})
		if n >= 3 && n <= 8 {
			require.NoError(rt, err, "len %d should pass", n)
		} else {
			require.Error(rt, err, "len %d should fail", n)
		}
	})
}

func ExampleStruct() {
	type Signup struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=8"`
		Plan     string `validate:"oneof=free pro"`
	}
	err := validator.Struct(Signup{Email: "bad", Password: "short", Plan: "enterprise"})
	fmt.Println(err != nil)
	// Output: true
}
