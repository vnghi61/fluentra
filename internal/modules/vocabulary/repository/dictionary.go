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
	baseURL         string
	datamuseURL     string
	client          *http.Client
	fallbackEnabled bool
}

const (
	freeDictionaryBaseURL = "https://api.dictionaryapi.dev/api/v2/entries/en"
	datamuseBaseURL       = "https://api.datamuse.com/words"
	dictionaryTimeout     = 5 * time.Second
	// The response for a common word is a few kilobytes; the cap is four
	// hundred times that and exists only so a misbehaving upstream cannot
	// exhaust memory.
	dictionaryMaxBytes = 2 << 20
)

// NewFreeDictionaryAPI builds the client. An empty baseURL uses the public API;
// tests pass their own server.
func NewFreeDictionaryAPI(baseURL string) *FreeDictionaryAPI {
	fallbackEnabled := false
	if strings.TrimSpace(baseURL) == "" {
		baseURL = freeDictionaryBaseURL
		fallbackEnabled = true
	}
	return &FreeDictionaryAPI{
		baseURL:         strings.TrimRight(baseURL, "/"),
		datamuseURL:     datamuseBaseURL,
		client:          &http.Client{Timeout: dictionaryTimeout},
		fallbackEnabled: fallbackEnabled,
	}
}

// WithDatamuseURL sets a custom Datamuse endpoint, primarily for testing.
func (d *FreeDictionaryAPI) WithDatamuseURL(u string) *FreeDictionaryAPI {
	d.datamuseURL = strings.TrimRight(u, "/")
	d.fallbackEnabled = true
	return d
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

type datamuseEntry struct {
	Word  string   `json:"word"`
	Score int      `json:"score"`
	Tags  []string `json:"tags"`
	Defs  []string `json:"defs"`
}

// Lookup asks the dictionary about one word.
func (d *FreeDictionaryAPI) Lookup(ctx context.Context, word string) (DictionaryEntry, error) {
	term := strings.TrimSpace(strings.ToLower(word))
	if term == "" {
		return DictionaryEntry{}, ErrWordNotFound
	}

	entry, err := d.lookupPrimary(ctx, term)
	if err == nil {
		return entry, nil
	}
	if errors.Is(err, ErrWordNotFound) {
		return DictionaryEntry{}, ErrWordNotFound
	}

	// If primary failed with a transport/server error (e.g. timeout or 5xx)
	// and fallback is enabled, fall back to Datamuse.
	if d.fallbackEnabled {
		fallbackEntry, fallbackErr := d.lookupDatamuse(ctx, term)
		if fallbackErr == nil {
			return fallbackEntry, nil
		}
		if errors.Is(fallbackErr, ErrWordNotFound) {
			return DictionaryEntry{}, ErrWordNotFound
		}
	}

	return DictionaryEntry{}, err
}

func (d *FreeDictionaryAPI) lookupPrimary(ctx context.Context, term string) (DictionaryEntry, error) {
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

func (d *FreeDictionaryAPI) lookupDatamuse(ctx context.Context, term string) (DictionaryEntry, error) {
	datamuseURL := d.datamuseURL
	if datamuseURL == "" {
		datamuseURL = datamuseBaseURL
	}
	endpoint := datamuseURL + "?sp=" + url.QueryEscape(term) + "&md=dpr&ipa=1"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return DictionaryEntry{}, fmt.Errorf("build datamuse request: %w", err)
	}

	response, err := d.client.Do(request)
	if err != nil {
		return DictionaryEntry{}, fmt.Errorf("call datamuse: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound {
		return DictionaryEntry{}, ErrWordNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return DictionaryEntry{}, fmt.Errorf("datamuse returned %d for %q", response.StatusCode, term)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, dictionaryMaxBytes))
	if err != nil {
		return DictionaryEntry{}, fmt.Errorf("read datamuse response: %w", err)
	}

	var items []datamuseEntry
	if err := json.Unmarshal(payload, &items); err != nil {
		return DictionaryEntry{}, fmt.Errorf("decode datamuse response: %w", err)
	}

	var match *datamuseEntry
	for i := range items {
		if strings.EqualFold(items[i].Word, term) {
			match = &items[i]
			break
		}
	}
	if match == nil {
		return DictionaryEntry{}, ErrWordNotFound
	}

	return mapDatamuseEntry(*match), nil
}

func mapDatamuseEntry(raw datamuseEntry) DictionaryEntry {
	entry := DictionaryEntry{
		Lemma: raw.Word,
	}

	for _, tag := range raw.Tags {
		if strings.HasPrefix(tag, "ipa_pron:") {
			rawIPA := strings.TrimPrefix(tag, "ipa_pron:")
			if rawIPA != "" && entry.IPA == "" {
				entry.IPA = "/" + rawIPA + "/"
			}
		} else if entry.PartOfSpeech == "" {
			if pos := mapDatamusePOS(tag); pos != "" {
				entry.PartOfSpeech = pos
			}
		}
	}

	for _, defStr := range raw.Defs {
		parts := strings.SplitN(defStr, "\t", 2)
		var pos, defText string
		if len(parts) == 2 {
			pos = mapDatamusePOS(parts[0])
			defText = strings.TrimSpace(parts[1])
		} else {
			defText = strings.TrimSpace(defStr)
		}

		if entry.Definition == "" && defText != "" {
			entry.Definition = defText
			if entry.PartOfSpeech == "" && pos != "" {
				entry.PartOfSpeech = pos
			}
		}
	}

	return entry
}

func mapDatamusePOS(tag string) string {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "n":
		return "noun"
	case "v":
		return "verb"
	case "adj":
		return "adjective"
	case "adv":
		return "adverb"
	case "prop":
		return "proper noun"
	default:
		return ""
	}
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
