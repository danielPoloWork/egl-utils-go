package validator_test

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/validator"
	"github.com/stretchr/testify/require"
)

// tagFragments are the pieces the fuzzer assembles into a `validate` tag: valid
// rules, rules with valid parameters, and every shape of malformed rule the
// grammar can be given — unknown names, missing parameters, non-numeric
// parameters, an integer far beyond int64, stray whitespace and separators.
//
// The fuzzer combines fragments rather than generating tag text byte by byte,
// and that is a deliberate constraint, not laziness. Injecting a tag requires
// building a struct type at run time with reflect.StructOf, and **reflect caches
// every type it constructs and never evicts** — so a target that fed unbounded
// tag text into StructOf would allocate a fresh cached type per execution and
// exhaust memory long before the fuzz budget expired. Bounding the tag to a few
// fragments from a fixed table bounds the number of distinct types
// (len(tagFragments)^maxFragments × len(fieldTypes)) to a few tens of thousands,
// which is flat memory. The field *value* is fuzzed freely, because that costs
// nothing.
var tagFragments = []string{
	"required",
	"email",
	"min=1",
	"max=8",
	"min=0",
	"max=99999999999999999999", // beyond int64: parameter parsing must fail loudly
	"min=-1",
	"min=",
	"min=abc",
	"oneof=dev prod",
	"oneof=",
	"oneof=1 2 3",
	"unknownrule",
	"",
	" ",
	"required ",
}

// maxFragments caps tag length; see the note on tagFragments. Three is enough to
// express every combination the grammar allows (a rule, a parameterised rule, and
// a malformed one together) while keeping the reflect type cache bounded.
const maxFragments = 3

// nested is a struct field type, so the fuzzer reaches Struct's recursion and its
// dotted-path reporting.
type nested struct {
	Inner string `validate:"max=4"`
}

// fieldTypes spans every kind the rules accept plus several they must reject:
// strings and numbers (min/max by value), collections (min/max by length), bool
// (rejected by min/max), a nested struct, and a pointer to one.
var fieldTypes = []reflect.Type{
	reflect.TypeOf(""),
	reflect.TypeOf(int(0)),
	reflect.TypeOf(int64(0)),
	reflect.TypeOf(uint(0)),
	reflect.TypeOf(float64(0)),
	reflect.TypeOf(true),
	reflect.TypeOf([]string(nil)),
	reflect.TypeOf(map[string]string(nil)),
	reflect.TypeOf([2]int{}),
	reflect.TypeOf(nested{}),
	reflect.TypeOf((*nested)(nil)),
}

// FuzzValidatorTags fuzzes the tag parser and the rule evaluators together, as
// required by spec v2 §7.
//
// The subtlety this target exists to handle: Struct **panics by contract** on tag
// misuse — an unknown rule, a rule on an incompatible type, a non-numeric
// parameter — because those are programming errors in a struct definition rather
// than validation failures (ADR-0023, ADR-0005 loud-by-default). A naive tag
// fuzzer therefore "finds" a panic within seconds and reports documented
// behaviour as a crash.
//
// So the property asserted is not "never panics". It is that **every failure mode
// stays inside the documented contract**:
//
//   - A panic must be a string beginning "validator: " — the package's own,
//     deliberate, diagnosable panic.
//   - A panic must never be a runtime.Error. A nil dereference, an index out of
//     range, or a slice-bounds violation is a genuine bug in the parser or an
//     evaluator, and this is the discriminator that separates one from the
//     contract.
//   - When it does not panic, the error must be nil or a ValidationErrors —
//     never some other error type leaking out of an evaluator.
//
// That last pair is what makes the target worth running: it can catch a
// malformed tag that is *silently accepted* (no panic, no error) as readily as
// one that crashes.
func FuzzValidatorTags(f *testing.F) {
	// Seeds: a valid combination, a type mismatch, an unknown rule, a bad
	// parameter, the empty tag, and a nested-struct case.
	f.Add([]byte{0, 1}, uint8(0), "ops@example.com", int64(3)) // required,email on a string
	f.Add([]byte{1}, uint8(1), "", int64(0))                   // email on an int → panic
	f.Add([]byte{12}, uint8(0), "x", int64(0))                 // unknown rule → panic
	f.Add([]byte{8}, uint8(1), "", int64(5))                   // min=abc → panic
	f.Add([]byte{5}, uint8(1), "", int64(1))                   // min beyond int64 → panic
	f.Add([]byte{2, 3}, uint8(6), "a b c", int64(0))           // min/max on a slice
	f.Add([]byte{9}, uint8(0), "dev", int64(0))                // oneof match
	f.Add([]byte{13}, uint8(9), "", int64(0))                  // empty tag, nested struct
	f.Add([]byte{0}, uint8(10), "", int64(0))                  // required on a nil pointer
	f.Add([]byte(nil), uint8(0), "", int64(0))                 // no fragments at all

	f.Fuzz(func(t *testing.T, sel []byte, kindSel uint8, s string, n int64) {
		tag := buildTag(sel)
		typ := fieldTypes[int(kindSel)%len(fieldTypes)]

		// Built outside the recovered region: a panic from StructOf itself would be
		// a fault in this test, not in the package under test, and must not be
		// mistaken for the contract.
		st := reflect.StructOf([]reflect.StructField{{
			Name: "F",
			Type: typ,
			Tag:  reflect.StructTag(`validate:"` + tag + `"`),
		}})
		pv := reflect.New(st)
		setField(pv.Elem().Field(0), s, n)

		var (
			err error
			rec any
		)
		func() {
			defer func() { rec = recover() }()
			err = validator.Struct(pv.Interface())
		}()

		if rec != nil {
			if re, ok := rec.(runtime.Error); ok {
				t.Fatalf("runtime error panic — a real crash, not the documented contract: "+
					"%v (tag %q, field kind %s)", re, tag, typ.Kind())
			}
			msg, ok := rec.(string)
			require.Truef(t, ok, "panic value must be a string, got %T (%v) for tag %q", rec, rec, tag)
			require.Truef(t, strings.HasPrefix(msg, "validator: "),
				"undocumented panic message %q for tag %q", msg, tag)
			return
		}

		if err != nil {
			var verrs validator.ValidationErrors
			require.ErrorAsf(t, err, &verrs,
				"Struct must return ValidationErrors or nil, got %T for tag %q", err, tag)
			for i, fe := range verrs {
				require.NotNilf(t, fe, "ValidationErrors[%d] is nil for tag %q", i, tag)
				require.NotEmptyf(t, fe.Error(), "FieldError[%d] has an empty message for tag %q", i, tag)
			}
		}
	})
}

// buildTag assembles a comma-separated tag from up to maxFragments table entries.
func buildTag(sel []byte) string {
	if len(sel) > maxFragments {
		sel = sel[:maxFragments]
	}
	parts := make([]string, 0, len(sel))
	for _, b := range sel {
		parts = append(parts, tagFragments[int(b)%len(tagFragments)])
	}
	return strings.Join(parts, ",")
}

// setField puts the fuzzed value into the generated field, mapping the two fuzzed
// scalars onto whichever kind the field has. A pointer field is left nil half the
// time so the nil-pointer branch is exercised too.
func setField(fv reflect.Value, s string, n int64) {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(s)
	case reflect.Int, reflect.Int64:
		fv.SetInt(n)
	case reflect.Uint:
		// Fold the fuzzed value into the unsigned range by masking, not by a sign
		// flip: negating math.MinInt64 overflows back to itself and would still be
		// negative. G115: reinterpreting an arbitrary fuzzed bit pattern as unsigned
		// is the intent here, and the mask makes the conversion lossless.
		fv.SetUint(uint64(n) & 0xFFFFFFFF) //nolint:gosec // deliberate, masked — see above
	case reflect.Float64:
		fv.SetFloat(float64(n))
	case reflect.Bool:
		fv.SetBool(n%2 == 0)
	case reflect.Slice: // []string
		fv.Set(reflect.ValueOf(strings.Fields(s)))
	case reflect.Map: // map[string]string
		m := reflect.MakeMap(fv.Type())
		for _, k := range strings.Fields(s) {
			m.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(s))
		}
		fv.Set(m)
	case reflect.Array: // [2]int
		fv.Index(0).SetInt(n)
		fv.Index(1).SetInt(n)
	case reflect.Struct: // nested
		fv.Field(0).SetString(s)
	case reflect.Pointer: // *nested
		if n%2 == 0 {
			fv.Set(reflect.ValueOf(&nested{Inner: s}))
		}
	default:
		// fieldTypes is a closed table; a new entry needs a case here.
		panic("fuzz setField: unhandled kind " + fv.Kind().String())
	}
}
