package main

import (
	"context"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/learning"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

type dummyGrader struct{}

func (dummyGrader) Grade(_ context.Context, _ learningcontract.GradeRequest) (learningcontract.GradeResult, error) {
	return learningcontract.GradeResult{}, nil
}

// TestGraders_CoversAllDeclaredKinds verifies that buildGraders populates every kind
// returned by buildDeclaredKinds.
func TestGraders_CoversAllDeclaredKinds(t *testing.T) {
	graders := buildGraders(dummyGrader{}, dummyGrader{})
	declared := buildDeclaredKinds()

	for _, kind := range declared {
		if _, ok := graders[kind]; !ok {
			t.Errorf("declared kind %q has no grader registered by composition root", kind)
		}
	}
}

// TestStartupPanic_WhenDeclaredKindMissingFromGraders proves that removing any declared
// kind from the composition root's grader map causes the learning module assembly to panic
// naming the missing kind at boot.
func TestStartupPanic_WhenDeclaredKindMissingFromGraders(t *testing.T) {
	declared := buildDeclaredKinds()
	if len(declared) == 0 {
		t.Fatal("no declared kinds returned by buildDeclaredKinds")
	}

	for _, missingKind := range declared {
		t.Run("missing_"+missingKind, func(t *testing.T) {
			graders := buildGraders(dummyGrader{}, dummyGrader{})
			delete(graders, missingKind)

			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected learning.New to panic when %q is missing from graders, but it did not", missingKind)
				}
				panicMsg := ""
				switch v := r.(type) {
				case error:
					panicMsg = v.Error()
				case string:
					panicMsg = v
				default:
					t.Fatalf("unexpected panic type: %T", r)
				}
				if !strings.Contains(panicMsg, missingKind) {
					t.Errorf("expected panic to name %q, got: %s", missingKind, panicMsg)
				}
			}()

			_ = learning.New(learning.Deps{
				Graders:       graders,
				DeclaredKinds: declared,
			})
		})
	}
}
