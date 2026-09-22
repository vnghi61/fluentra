package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
)

// A lesson_material is verified by structure only: it has no answer key and
// nothing to blind solve (WO 20 Stage B).
func TestVerifyItem_LessonMaterial(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.Deps{})

	// The draft shape: a resource id and a title.
	draftBody := json.RawMessage(`{"resource_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","title":"Intro"}`)
	require.NoError(t, svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind: learningcontract.KindLessonMaterial,
		Body: draftBody,
	}))

	// The published shape: object keys in place of the resource id.
	publishedBody := json.RawMessage(`{"material_kind":"video","title":"Intro","objects":{"original":"a.mp4"}}`)
	require.NoError(t, svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind: learningcontract.KindLessonMaterial,
		Body: publishedBody,
	}))

	// A material with no title is not authorable.
	err := svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind: learningcontract.KindLessonMaterial,
		Body: json.RawMessage(`{"resource_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "material title is required")

	// Neither a resource nor objects: nothing to show the learner.
	err = svc.VerifyItem(ctx, learningcontract.VerifyItemRequest{
		Kind: learningcontract.KindLessonMaterial,
		Body: json.RawMessage(`{"title":"Intro"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reference a resource")
}
