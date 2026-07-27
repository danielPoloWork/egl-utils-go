// Package validator validates struct values against rules declared in
// `validate:"..."` struct tags.
//
// The supported rules (spec §5) are:
//
//   - required          the field must not be its zero value
//   - email             the string must look like an email address
//   - min=N / max=N     for strings and slices/maps/arrays, the length (in
//     runes for strings) must be ≥/≤ N; for numbers, the
//     value must be ≥/≤ N
//   - oneof=a b c       the field's value must equal one of the space-separated
//     options
//
// Rules are comma-separated and applied literally in order — there is no
// implicit "optional": a field carrying min=3 must satisfy it whether or not it
// is also required. Struct recurses into nested struct fields (and non-nil
// pointers to structs), reporting a dotted path such as "Address.Zip".
//
// A rule that cannot apply to a field's type — email on a non-string, min on a
// bool, an unknown rule name, a non-numeric min/max parameter — is a
// programming error in the struct definition, so Struct panics (ADR-0005
// loud-by-default). The returned error reports only data that failed valid
// rules; it is never about a malformed tag.
package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// emailPattern is a pragmatic "looks like an email" check (a local part, an @,
// and a dotted domain), not an exhaustive RFC 5322 grammar. It rejects spaces
// and control characters and requires a dot in the domain.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// FieldError describes one field that failed one rule.
type FieldError struct {
	Field string // dotted path, e.g. "Address.Zip"
	Tag   string // the rule that failed, e.g. "min"
	Param string // the rule's parameter, e.g. "3" (empty for required/email)
}

func (e *FieldError) Error() string {
	if e.Param == "" {
		return fmt.Sprintf("field %q failed the %q rule", e.Field, e.Tag)
	}
	return fmt.Sprintf("field %q failed the %q rule (param %q)", e.Field, e.Tag, e.Param)
}

// ValidationErrors is the error returned by Struct when one or more fields fail
// validation. It joins its members' messages, and its Unwrap lets errors.As
// reach an individual *FieldError.
type ValidationErrors []*FieldError

func (es ValidationErrors) Error() string {
	msgs := make([]string, len(es))
	for i, e := range es {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// Unwrap exposes the individual field errors for errors.As / errors.Is.
func (es ValidationErrors) Unwrap() []error {
	errs := make([]error, len(es))
	for i, e := range es {
		errs[i] = e
	}
	return errs
}

// Struct validates v — a struct or a non-nil pointer to a struct — against its
// fields' `validate` tags, returning a ValidationErrors listing every failure
// or nil when all rules pass. It panics if v is nil, a nil pointer, or not a
// struct, and if a tag is malformed or applied to an incompatible type (see the
// package doc): those are programming errors, not validation failures.
func Struct(v any) error {
	if v == nil {
		panic("validator: nil value")
	}
	rv := deref(reflect.ValueOf(v))
	if rv.Kind() != reflect.Struct {
		panic(fmt.Sprintf("validator: expected a struct, got %s", rv.Kind()))
	}
	var errs ValidationErrors
	validateStruct(rv, "", &errs)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// deref follows pointers to the pointed-at value, panicking on a nil pointer
// (there is nothing to validate behind it).
func deref(rv reflect.Value) reflect.Value {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			panic("validator: nil pointer")
		}
		rv = rv.Elem()
	}
	return rv
}

// validateStruct applies each exported field's rules and recurses into nested
// structs, prefixing their field paths with prefix.
func validateStruct(rv reflect.Value, prefix string, errs *ValidationErrors) {
	t := rv.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		fv := rv.Field(i)
		name := prefix + sf.Name

		if tag := sf.Tag.Get("validate"); tag != "" && tag != "-" {
			applyRules(fv, name, tag, errs)
		}

		// Recurse into a nested struct or a non-nil pointer to one.
		nested := fv
		for nested.Kind() == reflect.Pointer && !nested.IsNil() {
			nested = nested.Elem()
		}
		if nested.Kind() == reflect.Struct {
			validateStruct(nested, name+".", errs)
		}
	}
}

// applyRules splits a tag into its comma-separated rules and records a
// FieldError for each that the field fails.
func applyRules(fv reflect.Value, name, tag string, errs *ValidationErrors) {
	for _, rule := range strings.Split(tag, ",") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		key, param, _ := strings.Cut(rule, "=")
		if !checkRule(fv, key, param) {
			*errs = append(*errs, &FieldError{Field: name, Tag: key, Param: param})
		}
	}
}

// checkRule reports whether fv satisfies one rule. It panics on a rule that
// cannot apply to fv's type or an unknown rule — a struct-definition bug.
func checkRule(fv reflect.Value, key, param string) bool {
	switch key {
	case "required":
		return !fv.IsZero()
	case "email":
		return checkEmail(fv)
	case "min":
		return checkMinMax(fv, param, true)
	case "max":
		return checkMinMax(fv, param, false)
	case "oneof":
		return checkOneOf(fv, param)
	default:
		panic(fmt.Sprintf("validator: unknown rule %q", key))
	}
}

func checkEmail(fv reflect.Value) bool {
	if fv.Kind() != reflect.String {
		panic(fmt.Sprintf("validator: 'email' rule on a non-string field (%s)", fv.Kind()))
	}
	return emailPattern.MatchString(fv.String())
}

// checkMinMax compares length (strings, collections) or value (numbers) against
// param. isMin selects ≥ (min) versus ≤ (max).
func checkMinMax(fv reflect.Value, param string, isMin bool) bool {
	switch fv.Kind() {
	case reflect.String:
		return cmpInt(int64(utf8.RuneCountInString(fv.String())), parseInt(param), isMin)
	case reflect.Slice, reflect.Map, reflect.Array:
		return cmpInt(int64(fv.Len()), parseInt(param), isMin)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cmpInt(fv.Int(), parseInt(param), isMin)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cmpUint(fv.Uint(), parseUint(param), isMin)
	case reflect.Float32, reflect.Float64:
		return cmpFloat(fv.Float(), parseFloat(param), isMin)
	default:
		panic(fmt.Sprintf("validator: 'min'/'max' rule on an unsupported kind (%s)", fv.Kind()))
	}
}

func cmpInt(v, threshold int64, isMin bool) bool {
	if isMin {
		return v >= threshold
	}
	return v <= threshold
}

func cmpUint(v, threshold uint64, isMin bool) bool {
	if isMin {
		return v >= threshold
	}
	return v <= threshold
}

func cmpFloat(v, threshold float64, isMin bool) bool {
	if isMin {
		return v >= threshold
	}
	return v <= threshold
}

// checkOneOf reports whether fv's scalar rendering equals one of the
// space-separated options.
func checkOneOf(fv reflect.Value, param string) bool {
	got := formatScalar(fv)
	for _, option := range strings.Fields(param) {
		if got == option {
			return true
		}
	}
	return false
}

func formatScalar(fv reflect.Value) string {
	switch fv.Kind() {
	case reflect.String:
		return fv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(fv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(fv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(fv.Float(), 'g', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(fv.Bool())
	default:
		panic(fmt.Sprintf("validator: 'oneof' rule on an unsupported kind (%s)", fv.Kind()))
	}
}

func parseInt(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("validator: invalid integer parameter %q", s))
	}
	return n
}

func parseUint(s string) uint64 {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("validator: invalid unsigned parameter %q", s))
	}
	return n
}

func parseFloat(s string) float64 {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Sprintf("validator: invalid float parameter %q", s))
	}
	return n
}
