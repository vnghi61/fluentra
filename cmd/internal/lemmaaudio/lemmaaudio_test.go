package lemmaaudio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// testLemma is the word every fixture looks up; a constant because goconst
// counts the repetitions.
const testLemma = "eat"

type stubDictionary struct {
	entry repository.DictionaryEntry
	err   error
}

func (s stubDictionary) Lookup(context.Context, string) (repository.DictionaryEntry, error) {
	return s.entry, s.err
}

// commonsServing answers the two file names the backfill asks for, as the real
// Commons does: 200 when the file exists and 404 when it does not.
func commonsServing(t *testing.T, existing ...string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, file := range existing {
			if strings.HasSuffix(r.URL.Path, file) {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func commonsBackfill(t *testing.T, dictionary repository.DictionaryLookup, existing ...string) *Lookup {
	t.Helper()
	audio := New(dictionary)
	audio.CommonsBase = commonsServing(t, existing...).URL + "/wiki/"
	audio.Client = &http.Client{}
	audio.Pace = 0
	return audio
}

func TestLookup_KeepsTheDictionarysRecording(t *testing.T) {
	// The dictionary is authoritative when it answers: it is the only source
	// that names the licence, and the Commons fallback exists for the days it
	// does not answer at all.
	audio := commonsBackfill(t, stubDictionary{entry: repository.DictionaryEntry{
		Lemma:            testLemma,
		AudioURL:         "https://api.dictionaryapi.dev/media/pronunciations/en/eat-us.mp3",
		AudioAttribution: "https://commons.wikimedia.org/wiki/File:En-us-eat.ogg",
		AudioLicence:     "BY-SA 3.0",
	}}, "En-us-eat.ogg")

	entry, found, err := audio.Lookup(context.Background(), testLemma)
	require.NoError(t, err)
	require.True(t, found)

	assert.Contains(t, entry.AudioURL, "api.dictionaryapi.dev")
	assert.Equal(t, "BY-SA 3.0", entry.AudioLicence)
}

func TestLookup_FallsBackToCommonsWhenTheDictionaryHasNoRecording(t *testing.T) {
	audio := commonsBackfill(t, stubDictionary{entry: repository.DictionaryEntry{Lemma: testLemma}},
		"En-us-eat.ogg")

	entry, found, err := audio.Lookup(context.Background(), testLemma)
	require.NoError(t, err)
	require.True(t, found)

	assert.Contains(t, entry.AudioURL, "Special:FilePath/En-us-eat.ogg")
	assert.Contains(t, entry.AudioAttribution, "File:En-us-eat.ogg")
	// The file page names the licence; the backfill does not guess it.
	assert.Empty(t, entry.AudioLicence)
}

func TestLookup_FallsBackToTheBritishFileWhenThereIsNoAmericanOne(t *testing.T) {
	audio := commonsBackfill(t, stubDictionary{entry: repository.DictionaryEntry{Lemma: testLemma}},
		"En-uk-eat.ogg")

	entry, found, err := audio.Lookup(context.Background(), testLemma)
	require.NoError(t, err)
	require.True(t, found)

	assert.Contains(t, entry.AudioURL, "En-uk-eat.ogg")
}

func TestLookup_SaysNothingWhenNeitherSourceHasTheWord(t *testing.T) {
	audio := commonsBackfill(t, stubDictionary{err: repository.ErrWordNotFound})

	_, found, err := audio.Lookup(context.Background(), "asdfgh")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestLookup_ReportsAFailureOnlyWhenCommonsHasNothingEither(t *testing.T) {
	// A dictionary timeout is not a verdict on the word. When Commons has no
	// file, the run must be able to say a lookup failed rather than claim the
	// word has no recording.
	audio := commonsBackfill(t, stubDictionary{err: assert.AnError})

	_, found, err := audio.Lookup(context.Background(), testLemma)
	require.Error(t, err)
	assert.False(t, found)
}
