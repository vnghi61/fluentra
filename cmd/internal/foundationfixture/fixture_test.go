package foundationfixture_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/cmd/internal/foundationfixture"
)

const testNamespace = "grammar"

func TestWriteThenReadAllRoundTrips(t *testing.T) {
	dir := t.TempDir()
	file := foundationfixture.File{
		Format:      foundationfixture.Format,
		GeneratedAt: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC),
		Node: foundationfixture.Node{
			Namespace: testNamespace, Code: "PRESENT_PERFECT", Label: "Present Perfect", CEFRLevel: "B1",
		},
		Items: []foundationfixture.Item{{
			Slug:      "foundation-topic-present-perfect",
			Kind:      "foundation_topic",
			CEFRLevel: "B1",
			Body:      json.RawMessage(`{"schema_version":1,"objective":"x"}`),
			Verification: foundationfixture.Verify{
				Confirmed: true, Model: "verifier-model",
			},
		}},
	}

	require.NoError(t, foundationfixture.Write(dir, file))

	files, err := foundationfixture.ReadAll(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, foundationfixture.Format, files[0].Format)
	assert.Equal(t, "PRESENT_PERFECT", files[0].Node.Code)
	require.Len(t, files[0].Items, 1)
	assert.True(t, files[0].Items[0].Verification.Confirmed)
}

func TestReadAll_RefusesAnUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, foundationfixture.Write(dir, foundationfixture.File{
		Format: "elsewhere.v1",
		Node:   foundationfixture.Node{Namespace: testNamespace, Code: "NOUNS"},
	}))

	_, err := foundationfixture.ReadAll(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown fixture format")
}

func TestValidate_RefusesAnItemWithNoBody(t *testing.T) {
	err := foundationfixture.Validate(foundationfixture.File{
		Format: foundationfixture.Format,
		Node:   foundationfixture.Node{Namespace: testNamespace, Code: "NOUNS"},
		Items:  []foundationfixture.Item{{Slug: "x", Kind: "foundation_quiz"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid body")
}
