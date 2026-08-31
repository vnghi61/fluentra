package repository_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// The shape api.dictionaryapi.dev actually returns, trimmed. Kept verbatim
// rather than minimised because the parts that matter here are the awkward
// ones: several phonetics blocks where only some carry audio, and a licence on
// the recording that the interface has to preserve.
const leisureResponse = `[{
  "word": "leisure",
  "phonetic": "/ˈliːʒə(ɹ)/",
  "phonetics": [
    {"text": "/ˈliːʒə(ɹ)/"},
    {"text": "/ˈliːʒəɹ/",
     "audio": "https://api.dictionaryapi.dev/media/pronunciations/en/leisure-ca-us.mp3",
     "sourceUrl": "https://commons.wikimedia.org/w/index.php?curid=424725",
     "license": {"name": "BY-SA 3.0"}}
  ],
  "meanings": [{
    "partOfSpeech": "noun",
    "definitions": [
      {"definition": "Free time, time free from work or duties.",
       "example": "He spends his leisure reading."},
      {"definition": "Freedom provided by the cessation of activities."}
    ]
  }]
}]`

func dictionaryServing(t *testing.T, status int, body string) *repository.FreeDictionaryAPI {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return repository.NewFreeDictionaryAPI(server.URL)
}

func TestFreeDictionary_ReadsTheIPAAndTheRecording(t *testing.T) {
	entry, err := dictionaryServing(t, http.StatusOK, leisureResponse).
		Lookup(context.Background(), "leisure")
	require.NoError(t, err)

	assert.Equal(t, "leisure", entry.Lemma)
	assert.Equal(t, "/ˈliːʒə(ɹ)/", entry.IPA)
	assert.Equal(t, "noun", entry.PartOfSpeech)
	assert.Equal(t, "Free time, time free from work or duties.", entry.Definition)

	// A URL, never a downloaded file: this is the whole point of using the
	// dictionary instead of an object store.
	assert.Equal(t,
		"https://api.dictionaryapi.dev/media/pronunciations/en/leisure-ca-us.mp3",
		entry.AudioURL)
}

func TestFreeDictionary_KeepsTheRecordingsAttribution(t *testing.T) {
	// The Wikimedia recordings are mostly CC BY-SA, and a licence that requires
	// attribution is not satisfied by playing the file. Dropping these fields
	// would leave the UI unable to credit anything.
	entry, err := dictionaryServing(t, http.StatusOK, leisureResponse).
		Lookup(context.Background(), "leisure")
	require.NoError(t, err)

	assert.Equal(t, "https://commons.wikimedia.org/w/index.php?curid=424725", entry.AudioAttribution)
	assert.Equal(t, "BY-SA 3.0", entry.AudioLicence)
}

func TestFreeDictionary_SkipsPhoneticsBlocksWithNoAudio(t *testing.T) {
	// The first block has an IPA and no audio; taking it wholesale would leave
	// the word silent even though a recording exists two entries down.
	entry, err := dictionaryServing(t, http.StatusOK, leisureResponse).
		Lookup(context.Background(), "leisure")
	require.NoError(t, err)
	assert.NotEmpty(t, entry.AudioURL)
}

func TestFreeDictionary_CollectsTheDictionarysOwnExamples(t *testing.T) {
	entry, err := dictionaryServing(t, http.StatusOK, leisureResponse).
		Lookup(context.Background(), "leisure")
	require.NoError(t, err)
	assert.Equal(t, []string{"He spends his leisure reading."}, entry.Examples)
}

func TestFreeDictionary_NotFoundIsAVerdict(t *testing.T) {
	// "This is not a word" is something the job acts on. It must be
	// distinguishable from "the dictionary was unreachable", which is a reason
	// to try again next hour rather than to reject a learner's good word.
	_, err := dictionaryServing(t, http.StatusNotFound, `{"title":"No Definitions Found"}`).
		Lookup(context.Background(), "asdfgh")
	assert.ErrorIs(t, err, repository.ErrWordNotFound)
}

func TestFreeDictionary_AFailureIsNotAVerdict(t *testing.T) {
	_, err := dictionaryServing(t, http.StatusBadGateway, "upstream is down").
		Lookup(context.Background(), "leisure")

	require.Error(t, err)
	assert.NotErrorIs(t, err, repository.ErrWordNotFound,
		"a bad gateway must never be read as 'not a word'")
}

func TestFreeDictionary_AnEmptyArrayIsNotFound(t *testing.T) {
	_, err := dictionaryServing(t, http.StatusOK, `[]`).Lookup(context.Background(), "x")
	assert.ErrorIs(t, err, repository.ErrWordNotFound)
}

func TestFreeDictionary_RejectsABlankTermWithoutCallingOut(t *testing.T) {
	// No server at all: a blank term must not produce a request.
	client := repository.NewFreeDictionaryAPI("http://127.0.0.1:0")
	_, err := client.Lookup(context.Background(), "   ")
	assert.ErrorIs(t, err, repository.ErrWordNotFound)
}
