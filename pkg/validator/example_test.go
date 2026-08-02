package validator_test

import (
	"errors"
	"fmt"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/validator"
)

// Struct applies the rules declared in each field's `validate` tag and returns
// nil when they all pass.
//
// It prints whether an error occurred rather than the error's text, on purpose:
// whatever an example prints, go test enforces from then on, and pinning the
// wording of a message would make improving it a breaking change. The next
// example shows how to read the failures structurally, which is what a caller
// should do too.
func ExampleStruct() {
	type Signup struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=8"`
		Plan     string `validate:"oneof=free pro"`
	}

	err := validator.Struct(Signup{Email: "bad", Password: "short", Plan: "enterprise"})
	fmt.Println(err != nil)

	err = validator.Struct(Signup{Email: "ada@example.com", Password: "correct-horse", Plan: "pro"})
	fmt.Println(err == nil)
	// Output:
	// true
	// true
}

// Struct reports every field that failed, not just the first, so a form or a
// config file can be corrected in one round trip. Type-assert the error to
// ValidationErrors — or reach a single failure with errors.As — instead of
// parsing the message.
func ExampleValidationErrors() {
	type Signup struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=8"`
		Plan     string `validate:"oneof=free pro"`
	}

	err := validator.Struct(Signup{Email: "bad", Password: "short", Plan: "enterprise"})

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		// Reported in field-declaration order, and each entry carries the rule
		// that failed and its parameter — enough to build a per-field message in
		// the caller's own wording and language.
		for _, fe := range verrs {
			fmt.Printf("%s %s %q\n", fe.Field, fe.Tag, fe.Param)
		}
	}

	// Unwrap also exposes the members individually, so errors.As reaches a
	// single *FieldError when only one of them matters.
	var fe *validator.FieldError
	fmt.Println(errors.As(err, &fe), fe.Field)
	// Output:
	// Email email ""
	// Password min "8"
	// Plan oneof "free pro"
	// true Email
}

// Nested structs are validated too, and a failure inside one is reported with a
// dotted path — so the caller learns which field of which sub-struct failed
// without the outer type having to restate its children's rules.
func ExampleStruct_nested() {
	type Address struct {
		City string `validate:"required"`
		Zip  string `validate:"required,min=5,max=5"`
	}
	type Customer struct {
		Name    string `validate:"required"`
		Billing Address
	}

	err := validator.Struct(Customer{Name: "Ada", Billing: Address{City: "Lovelace", Zip: "123"}})

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, fe := range verrs {
			fmt.Println(fe.Field)
		}
	}
	// Output: Billing.Zip
}

// Rules apply literally and in order — there is no implicit "optional". A field
// tagged min=3 must satisfy it whether or not it is also required, so an empty
// optional string fails a bare min. Express "empty or at least 3" with a
// pointer field and a Validator on the enclosing type, not with a tag.
func ExampleStruct_rulesApplyLiterally() {
	type Search struct {
		Query string `validate:"min=3"`
	}

	fmt.Println(validator.Struct(Search{Query: ""}) != nil)
	fmt.Println(validator.Struct(Search{Query: "go"}) != nil)
	fmt.Println(validator.Struct(Search{Query: "golang"}) != nil)
	// Output:
	// true
	// true
	// false
}
