package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DictionaryEntry is what a free dictionary knows about a word.
//
// Deliberately the subset the verification job uses. The upstream response
// carries synonyms, antonyms, etymology and several senses; taking all of it
// would make this type a mirror of somebody else's API, and every change to
// theirs a change to ours.
type DictionaryEntry struct {
	Lemma        string
	IPA          string
	PartOfSpeech string
	Definition   string
	// AudioURL is a recording of a person saying the word, hosted upstream.
	//
	// A URL, never a file. Storing the audio would mean an object store, a
	// lifecycle policy, a licence audit and a few hundred megabytes, to hold a
	// copy of something already served for free — and the browser's own speech
	// synthesis already covers every word for which no recording exists. What
	// this buys over synthesis is a human voice, which is worth a URL and not
	// much more.
	AudioURL string
	// AudioAttribution is the source page for the recording.
	//
	// Not decoration: the Wikimedia Commons recordings this API serves are
	// mostly CC BY-SA, and a licence that requires attribution is not satisfied
	// by playing the file. A UI that plays the audio must be able to credit it,
	// so the link is carried rather than discarded.
	AudioAttribution string
	AudioLicence     string
	// Examples the dictionary itself supplies, where it has any. Usually few,
	// which is why the job asks a model for the rest.
	Examples []string
}

// DictionaryLookup resolves a word against an external dictionary.
//
// An interface so the verification service can be tested without a network, and
// so a different dictionary can be put behind it without the service noticing.
type DictionaryLookup interface {
	// Lookup returns the entry, or ErrWordNotFound when the dictionary has no
	// entry for the word. Any other error is a failure to ask, not an answer.
	Lookup(ctx context.Context, word string) (DictionaryEntry, error)
}

// ErrWordNotFound reports that the dictionary has no entry.
//
// Distinct from a transport failure on purpose: "this is not a word" is a
// verdict the job acts on, and "the dictionary was unreachable" is a reason to
// leave the item pending and try again next hour. Conflating them would mark a
// learner's perfectly good word invalid because of a network blip.
var ErrWordNotFound = errors.New("vocabulary: no dictionary entry")

// FreeDictionaryAPI reads api.dictionaryapi.dev.
//
// Chosen because it needs no key, no account and no billing relationship, and
// because it returns the two things a flashcard wants and the seed never had: an
// IPA transcription, and a link to a human pronunciation.
type FreeDictionaryAPI struct {
	baseURL string
	client  *http.Client
}

const (
	freeDictionaryBaseURL = "https://api.dictionaryapi.dev/api/v2/entries/en"
	dictionaryTimeout     = 15 * time.Second
	// The response for a common word is a few kilobytes; the cap is four
	// hundred times that and exists only so a misbehaving upstream cannot
	// exhaust memory.
	dictionaryMaxBytes = 2 << 20
)

// NewFreeDictionaryAPI builds the client. An empty baseURL uses the public API;
// tests pass their own server.
func NewFreeDictionaryAPI(baseURL string) *FreeDictionaryAPI {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = freeDictionaryBaseURL
	}
	return &FreeDictionaryAPI{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: dictionaryTimeout},
	}
}

// The upstream response, named for what it is rather than mapped field by field
// into the domain type: the shape belongs to them.
type dictionaryAPIEntry struct {
	Word      string `json:"word"`
	Phonetic  string `json:"phonetic"`
	Phonetics []struct {
		Text      string `json:"text"`
		Audio     string `json:"audio"`
		SourceURL string `json:"sourceUrl"`
		License   *struct {
			Name string `json:"name"`
		} `json:"license"`
	} `json:"phonetics"`
	Meanings []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string `json:"definition"`
			Example    string `json:"example"`
		} `json:"definitions"`
	} `json:"meanings"`
}

// Lookup asks the dictionary about one word.
func (d *FreeDictionaryAPI) Lookup(ctx context.Context, word string) (DictionaryEntry, error) {
	term := strings.TrimSpace(strings.ToLower(word))
	if term == "" {
		return DictionaryEntry{}, ErrWordNotFound
	}

	endpoint := d.baseURL + "/" + url.PathEscape(term)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return DictionaryEntry{}, fmt.Errorf("build dictionary request: %w", err)
	}

	response, err := d.client.Do(request)
	if err != nil {
		return DictionaryEntry{}, fmt.Errorf("call dictionary: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	// 404 is the documented "no such word" answer, and is the verdict the job
	// wants rather than an error to retry.
	if response.StatusCode == http.StatusNotFound {
		return DictionaryEntry{}, ErrWordNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return DictionaryEntry{}, fmt.Errorf("dictionary returned %d for %q", response.StatusCode, term)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, dictionaryMaxBytes))
	if err != nil {
		return DictionaryEntry{}, fmt.Errorf("read dictionary response: %w", err)
	}

	var entries []dictionaryAPIEntry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return DictionaryEntry{}, fmt.Errorf("decode dictionary response: %w", err)
	}
	if len(entries) == 0 {
		return DictionaryEntry{}, ErrWordNotFound
	}
	return mapDictionaryEntry(entries[0]), nil
}

// mapDictionaryEntry takes the first usable value for each field.
//
// The upstream returns several phonetics blocks, most of them duplicates and
// some with no audio at all, and several meanings. First-usable rather than
// best-match because there is no signal to rank them by, and because a
// flashcard needs one of each rather than the right one of many.
func mapDictionaryEntry(raw dictionaryAPIEntry) DictionaryEntry {
	entry := DictionaryEntry{
		Lemma: raw.Word,
		IPA:   raw.Phonetic,
	}

	for _, phonetic := range raw.Phonetics {
		if entry.IPA == "" && phonetic.Text != "" {
			entry.IPA = phonetic.Text
		}
		if entry.AudioURL == "" && phonetic.Audio != "" {
			entry.AudioURL = phonetic.Audio
			entry.AudioAttribution = phonetic.SourceURL
			if phonetic.License != nil {
				entry.AudioLicence = phonetic.License.Name
			}
		}
	}

	for _, meaning := range raw.Meanings {
		for _, definition := range meaning.Definitions {
			if entry.Definition == "" && definition.Definition != "" {
				entry.Definition = definition.Definition
				entry.PartOfSpeech = meaning.PartOfSpeech
			}
			if definition.Example != "" {
				entry.Examples = append(entry.Examples, definition.Example)
			}
		}
	}
	return entry
}

var _ DictionaryLookup = (*FreeDictionaryAPI)(nil)
