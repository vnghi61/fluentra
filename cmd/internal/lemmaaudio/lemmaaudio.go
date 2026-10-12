// Package lemmaaudio looks up a lemma's recorded pronunciation: the free
// dictionary first, Wikimedia Commons by its file-naming convention after it.
//
// Shared by the seed's audio repair (`cmd/seed -audio`) and the vocabulary
// build tool's pronunciation step (`cmd/vocabgen -pronounce`, WO 22 Stage C
// step 3), so both choose the same recording for the same word.
package lemmaaudio

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// Pace is how long to wait between two dictionary lookups.
//
// The API is free, keyless and somebody else's. A few hundred words at four
// lookups a second is under a minute and still a well-behaved client.
const Pace = 250 * time.Millisecond

// UserAgent is who the backfill says it is.
//
// Wikimedia refuses the Go client's default User-Agent with a 403 — its policy
// requires a descriptive one. A browser sends one, which is why the browser
// fallback works and this backfill returned nothing until it introduced itself.
const UserAgent = "FluentraSeed/0.1 (+https://github.com/fluentra/fluentra)"

// CommonsBaseURL is where Wikimedia Commons lives. A field on Lookup so a
// test can point it at its own server.
const CommonsBaseURL = "https://commons.wikimedia.org/wiki/"

// Lookup looks a lemma up once per run.
//
// Several senses share a lemma ("bank" the noun and the verb), and so do the
// lesson flashcards and the word sense they were built from. One lookup per
// lemma is the difference between 220 requests and about 200, and it is the
// only way the two paths can be guaranteed to choose the same recording.
type Lookup struct {
	dictionary repository.DictionaryLookup
	// Client, CommonsBase and Pace are fields so a test can point the lookup at
	// its own server and not wait.
	Client      *http.Client
	CommonsBase string
	Pace        time.Duration
	entries     map[string]repository.DictionaryEntry
	errs        map[string]error
}

// New returns a lookup that asks the dictionary first and Commons after it.
func New(dictionary repository.DictionaryLookup) *Lookup {
	return &Lookup{
		dictionary:  dictionary,
		Client:      &http.Client{Timeout: 5 * time.Second},
		CommonsBase: CommonsBaseURL,
		Pace:        Pace,
		entries:     make(map[string]repository.DictionaryEntry),
		errs:        make(map[string]error),
	}
}

// Lookup returns the dictionary entry for a lemma, remembering both answers and
// failures. found is false when neither source has anything.
func (l *Lookup) Lookup(ctx context.Context, lemma string) (repository.DictionaryEntry, bool, error) {
	key := strings.ToLower(strings.TrimSpace(lemma))
	if entry, ok := l.entries[key]; ok {
		return entry, true, nil
	}
	if err, ok := l.errs[key]; ok {
		return repository.DictionaryEntry{}, false, err
	}

	entry, err := l.dictionary.Lookup(ctx, key)
	switch {
	case err == nil:
		if entry.AudioURL == "" {
			entry = l.withCommonsRecording(ctx, key, entry)
		}
	case errors.Is(err, repository.ErrWordNotFound):
		// The dictionary does not know the word; Commons may still hold a
		// recording of it, and the file page is still a credit.
		commons, found := l.commonsRecording(ctx, key)
		if !found {
			l.entries[key] = repository.DictionaryEntry{}
			return repository.DictionaryEntry{}, false, nil
		}
		entry = commons
	default:
		// A transport failure is the dictionary being unavailable, not a
		// verdict on the word. Commons is the source of last resort, exactly
		// as it is in the browser.
		commons, found := l.commonsRecording(ctx, key)
		if !found {
			l.errs[key] = err
			return repository.DictionaryEntry{}, false, err
		}
		entry = commons
	}

	l.entries[key] = entry
	// The dictionary's courtesy pace, paid once per real request.
	time.Sleep(l.Pace)
	return entry, true, nil
}

// withCommonsRecording fills in the audio the dictionary did not return.
func (l *Lookup) withCommonsRecording(
	ctx context.Context, lemma string, entry repository.DictionaryEntry,
) repository.DictionaryEntry {
	commons, found := l.commonsRecording(ctx, lemma)
	if !found {
		return entry
	}
	entry.AudioURL = commons.AudioURL
	entry.AudioAttribution = commons.AudioAttribution
	entry.AudioLicence = ""
	return entry
}

// commonsRecording looks the word up in Wikimedia Commons by its file-naming
// convention: `En-us-<word>.ogg`, then `En-uk-<word>.ogg`.
//
// The same two names the browser tries, and for the same reason: the dictionary
// is a free service that is sometimes slow enough to time out (measured at 20 s
// per request on 2026-09-22), and the recording is still there. The file page
// is the credit CC BY-SA requires; the licence's short name is not on it, so
// none is claimed.
func (l *Lookup) commonsRecording(ctx context.Context, lemma string) (repository.DictionaryEntry, bool) {
	commons := l.CommonsBase
	for _, prefix := range []string{"En-us", "En-uk"} {
		file := fmt.Sprintf("%s-%s.ogg", prefix, lemma)
		fileURL := commons + "Special:FilePath/" + url.PathEscape(file)
		request, err := http.NewRequestWithContext(ctx, http.MethodHead, fileURL, nil)
		if err != nil {
			continue
		}
		request.Header.Set("User-Agent", UserAgent)
		response, err := l.Client.Do(request)
		if err != nil {
			continue
		}
		_ = response.Body.Close()
		// A missing file is a 404; Special:FilePath answers 302 to the file
		// itself, and the client follows it.
		if response.StatusCode < 200 || response.StatusCode >= 400 {
			continue
		}
		return repository.DictionaryEntry{
			Lemma:            lemma,
			AudioURL:         fileURL,
			AudioAttribution: commons + "File:" + url.PathEscape(file),
		}, true
	}
	return repository.DictionaryEntry{}, false
}
