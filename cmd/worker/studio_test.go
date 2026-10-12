package main

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	resourcecontract "github.com/fluentra/fluentra/internal/modules/resource/contract"
)

// The worker assembles studio before the resource module, so studio receives a
// late holder for the read-and-copy surface. It must satisfy the contract and
// fail closed until the module exists — never answer as an empty success.
var _ resourcecontract.MaterialPublisher = (*lateResource)(nil)

func TestLateResourceMaterialPublisherFailsClosedBeforeAssembly(t *testing.T) {
	holder := &lateResource{}
	ctx := context.Background()

	if _, err := holder.MaterialForOwner(ctx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected an error before the resource module is assembled")
	} else if !strings.Contains(err.Error(), "not assembled") {
		t.Fatalf("error should say the module is not assembled, got %v", err)
	}

	if _, err := holder.CopyForPublication(ctx, uuid.New(), "course-materials/prefix/"); err == nil {
		t.Fatal("expected an error before the resource module is assembled")
	} else if !strings.Contains(err.Error(), "not assembled") {
		t.Fatalf("error should say the module is not assembled, got %v", err)
	}

	if _, err := holder.GetResource(ctx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected the resource read to fail closed before assembly")
	}
}
