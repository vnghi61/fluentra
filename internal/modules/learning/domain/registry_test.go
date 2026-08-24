package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

func TestGraderRegistry_RegisterAndGet(t *testing.T) {
	registry := domain.NewGraderRegistry()
	fakeGrader := domain.NewFakeGrader()

	if err := registry.Register("quiz", fakeGrader); err != nil {
		t.Fatalf("Register quiz: %v", err)
	}

	grader, ok := registry.Get("quiz")
	if !ok || grader == nil {
		t.Fatalf("expected grader for quiz, got ok=%v", ok)
	}

	_, ok = registry.Get("unregistered")
	if ok {
		t.Errorf("expected unregistered kind to return ok=false")
	}
}

func TestGraderRegistry_RegisterInvalid(t *testing.T) {
	registry := domain.NewGraderRegistry()

	if err := registry.Register("", domain.NewFakeGrader()); err == nil {
		t.Errorf("expected error registering empty kind")
	}

	if err := registry.Register("quiz", nil); err == nil {
		t.Errorf("expected error registering nil grader")
	}
}

func TestGraderRegistry_ValidateSuccess(t *testing.T) {
	registry := domain.NewGraderRegistry()
	_ = registry.Register("quiz", domain.NewFakeGrader())
	_ = registry.Register("multiple_choice", domain.NewFakeGrader())

	err := registry.Validate([]string{"quiz", "multiple_choice"})
	if err != nil {
		t.Fatalf("expected validation success, got: %v", err)
	}
}

func TestGraderRegistry_ValidateMissingKindNamesTheKind(t *testing.T) {
	registry := domain.NewGraderRegistry()
	_ = registry.Register("quiz", domain.NewFakeGrader())

	err := registry.Validate([]string{"quiz", "reading_mcq"})
	if err == nil {
		t.Fatal("expected validation failure for missing kind reading_mcq")
	}

	if !strings.Contains(err.Error(), "reading_mcq") {
		t.Errorf("expected error to name missing kind 'reading_mcq', got: %v", err)
	}
}

func TestFakeGrader_Grade(t *testing.T) {
	grader := domain.NewFakeGrader()
	res, err := grader.Grade(context.Background(), contract.GradeRequest{})
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if res.Score != 100 || res.MaxScore != 100 || !res.Correct || res.Async {
		t.Errorf("unexpected GradeResult: %+v", res)
	}
}

func TestAsyncFakeGrader_Grade(t *testing.T) {
	grader := domain.NewAsyncFakeGrader()
	res, err := grader.Grade(context.Background(), contract.GradeRequest{})
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if !res.Async {
		t.Errorf("expected Async: true, got %+v", res)
	}
}
