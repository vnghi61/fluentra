package repository_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestFreeDictionary_FallsBackToDatamuseWhenPrimaryFails(t *testing.T) {
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer primaryServer.Close()

	datamuseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "habit", r.URL.Query().Get("sp"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{` +
			`"word":"habit",` +
			`"tags":["n","v","ipa_pron:hˈæbʌt"],` +
			`"defs":["n\tAn action performed on a regular basis."]` +
			`}]`))
	}))
	defer datamuseServer.Close()

	client := repository.NewFreeDictionaryAPI(primaryServer.URL).WithDatamuseURL(datamuseServer.URL)
	entry, err := client.Lookup(context.Background(), "habit")
	require.NoError(t, err)

	assert.Equal(t, "habit", entry.Lemma)
	assert.Equal(t, "/hˈæbʌt/", entry.IPA)
	assert.Equal(t, "noun", entry.PartOfSpeech)
	assert.Equal(t, "An action performed on a regular basis.", entry.Definition)
}

func TestFreeDictionary_FallsBackToDatamuseAndReturnsNotFoundWhenMissing(t *testing.T) {
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream timeout", http.StatusGatewayTimeout)
	}))
	defer primaryServer.Close()

	datamuseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer datamuseServer.Close()

	client := repository.NewFreeDictionaryAPI(primaryServer.URL).WithDatamuseURL(datamuseServer.URL)
	_, err := client.Lookup(context.Background(), "nonexistentwordxyz")
	assert.ErrorIs(t, err, repository.ErrWordNotFound)
}

// TestFreeDictionary_LiveDatamuseFallback calls api.datamuse.com for real, and
// is opt-in for that reason.
//
// `testing.Short()` was the wrong guard. The fast loop passes -short, but CI
// runs `go test -race -tags=integration ./...` without it, so this reached the
// live service on every run — and would have passed almost always, which is the
// worse kind of flake: a red build with no relation to the change that
// triggered it, and no obvious cause when someone goes looking. Run it on
// purpose instead:
//
//	DICTIONARY_LIVE_TEST=1 go test ./internal/modules/vocabulary/repository/ -run Live
func TestFreeDictionary_LiveDatamuseFallback(t *testing.T) {
	if os.Getenv("DICTIONARY_LIVE_TEST") == "" {
		t.Skip("set DICTIONARY_LIVE_TEST=1 to call the live Datamuse API")
	}
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer primaryServer.Close()

	client := repository.NewFreeDictionaryAPI(primaryServer.URL).WithDatamuseURL("https://api.datamuse.com/words")
	entry, err := client.Lookup(context.Background(), "work")
	require.NoError(t, err)
	assert.Equal(t, "work", entry.Lemma)
	assert.NotEmpty(t, entry.Definition)
	assert.NotEmpty(t, entry.PartOfSpeech)
}

func TestFreeDictionary_FindCandidates_Fixtures(t *testing.T) {
	// Test candidate retrieval for schol, recieve, form using recorded fixtures
	datamuseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		sp := r.URL.Query().Get("sp")
		sug := r.URL.Query().Get("s")

		var filename string
		if path == "/words" && sp != "" {
			filename = "testdata/datamuse_sp_" + sp + ".json"
		} else if path == "/sug" && sug != "" {
			filename = "testdata/datamuse_sug_" + sug + ".json"
		}

		if filename != "" {
			if data, err := os.ReadFile(filename); err == nil {
				_, _ = w.Write(data)
				return
			}
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer datamuseServer.Close()

	client := repository.NewFreeDictionaryAPI("http://127.0.0.1:0").
		WithDatamuseURL(datamuseServer.URL + "/words")

	// 1. schol: sp fixture contains "school" (distance 1 <= 2)
	cands, err := client.FindCandidates(context.Background(), "schol")
	require.NoError(t, err)
	assert.Contains(t, cands, "school")

	// 2. form: sp fixture contains only "form" (same as term, filtered out)
	// and sug fixture contains no close spellings (formidable, formulate, etc. are distance > 1)
	cands, err = client.FindCandidates(context.Background(), "form")
	require.NoError(t, err)
	assert.Empty(t, cands)

	// 3. recieve: sp fixture contains "relieve", "decieve" (distance 2 and 3; bound for 7 letters is 2, so relieve is distance 2, receive is not in fixture)
	cands, err = client.FindCandidates(context.Background(), "recieve")
	require.NoError(t, err)
	// relieve is distance 2 from recieve (transposition c<->l? No: c->l is substitution, so r-e-c-i-e-v-e vs r-e-l-i-e-v-e: 1 substitution. Distance is 1 <= 2!)
	assert.Contains(t, cands, "relieve")
}
