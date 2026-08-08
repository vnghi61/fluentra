package validation

import "testing"

func TestValidator_Check(t *testing.T) {
	t.Parallel()
	input := struct {
		Email    string `validate:"required,email"`
		Password string `validate:"min=12"`
	}{Email: "not-email", Password: "short"}
	errors := New().Check(input, "en")
	if len(errors) != 2 || errors[0].Field != "email" || errors[1].Code != "min" {
		t.Fatalf("errors = %#v", errors)
	}
	if valid := New().Check(struct {
		Email string `validate:"required,email"`
	}{Email: "ada@example.com"}, "vi"); len(valid) != 0 {
		t.Fatalf("valid errors = %#v", valid)
	}
}

func TestValidator_CheckVietnameseMessage(t *testing.T) {
	t.Parallel()
	input := struct {
		Email string `validate:"required,email"`
	}{}
	errors := New().Check(input, "vi")
	if len(errors) != 1 || errors[0].Message == "" {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestValidator_CheckUnknownValidationError(t *testing.T) {
	t.Parallel()
	errors := New().Check(nil, "en")
	if len(errors) != 1 || errors[0].Code != "invalid" {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestValidator_CheckVietnameseEmailAndMin(t *testing.T) {
	t.Parallel()
	input := struct {
		Email    string `validate:"email"`
		Password string `validate:"min=12"`
	}{Email: "bad", Password: "short"}
	errors := New().Check(input, "vi")
	if len(errors) != 2 || errors[0].Message == "" || errors[1].Message == "" {
		t.Fatalf("errors = %#v", errors)
	}
}
