// Command vocabgen builds the vocabulary fixture from an openly licensed
// frequency list (WO 22 Stage C).
//
// Four resumable steps: read and filter the list, write meanings with the model
// fifty lemmas per call, look up pronunciation, and write one file per CEFR
// level under db/fixtures/vocabulary. Each step caches to a scratch file, so a
// crash at word 7,000 does not start over.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/vocabfixture"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/config"
)

const (
	defaultFixtureDir = "db/fixtures/vocabulary"
	defaultCacheFile  = "vocabgen-cache.json"
	defaultLimit      = 10000
)

type vocabCLIConfig struct {
	App struct {
		Environment string `koanf:"environment"`
	} `koanf:"app"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	AI struct {
		Provider1Name    string        `koanf:"provider_1_name"`
		Provider1BaseURL string        `koanf:"provider_1_base_url"`
		Provider1Model   string        `koanf:"provider_1_model"`
		Provider1APIKey  string        `koanf:"provider_1_api_key"`
		Provider1Timeout time.Duration `koanf:"provider_1_timeout"`
		Provider2Name    string        `koanf:"provider_2_name"`
		Provider2BaseURL string        `koanf:"provider_2_base_url"`
		Provider2Model   string        `koanf:"provider_2_model"`
		Provider2APIKey  string        `koanf:"provider_2_api_key"`
		Provider2Timeout time.Duration `koanf:"provider_2_timeout"`
		Provider3Name    string        `koanf:"provider_3_name"`
		Provider3BaseURL string        `koanf:"provider_3_base_url"`
		Provider3Model   string        `koanf:"provider_3_model"`
		Provider3APIKey  string        `koanf:"provider_3_api_key"`
		Provider3Timeout time.Duration `koanf:"provider_3_timeout"`
		Provider4Name    string        `koanf:"provider_4_name"`
		Provider4BaseURL string        `koanf:"provider_4_base_url"`
		Provider4Model   string        `koanf:"provider_4_model"`
		Provider4APIKey  string        `koanf:"provider_4_api_key"`
		Provider4Timeout time.Duration `koanf:"provider_4_timeout"`
		AutoPublish      bool          `koanf:"auto_publish"`
	} `koanf:"ai"`
}

// modelMeaning is one headword as the model returns it.
type modelMeaning struct {
	Lemma        string                 `json:"lemma"`
	POS          string                 `json:"pos"`
	CEFRLevel    string                 `json:"cefr_level"`
	Definition   string                 `json:"definition"`
	DefinitionVI string                 `json:"definition_vi"`
	Examples     []vocabfixture.Example `json:"examples"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "vocabgen error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("vocabgen", flag.ContinueOnError)
	sourceFlag := flags.String("source", "", "The frequency list: one lemma per line, or rank<TAB>lemma")
	limitFlag := flags.Int("limit", defaultLimit, "How many lemmas to build")
	fixturesFlag := flags.String("fixtures", defaultFixtureDir,
		"Directory the level files are written to")
	cacheFlag := flags.String("cache", defaultCacheFile,
		"Scratch file holding the meanings written so far, so a run resumes")
	batchFlag := flags.Int("batch", 15, "Lemmas per model call")
	dryRunFlag := flags.Bool("dry-run", false, "Print what would be built without writing")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceFlag) == "" {
		return errors.New("must specify -source FILE, the openly licensed frequency list")
	}

	cfg, err := loadConfig(ctx)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	client := initAIClient(ctx, cfg, pool)
	if _, isMock := client.(*ai.MockProvider); isMock {
		return errors.New("no AI provider is configured; set AI_PROVIDER_1_NAME and AI_PROVIDER_1_API_KEY")
	}

	lemmas, err := readSource(*sourceFlag, *limitFlag)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Read %d lemma(s) from %s\n", len(lemmas), *sourceFlag)

	cache, err := readCache(*cacheFlag)
	if err != nil {
		return err
	}
	// The fixtures already written are part of the cache: a run that stopped
	// after writing words 1-3,000 resumes at 3,001 without paying for them
	// twice, even if the scratch cache is gone.
	if err := mergeCacheFromFixtures(*fixturesFlag, cache); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Cache holds %d meaning(s)\n", len(cache))

	batchSize := *batchFlag
	if batchSize <= 0 {
		batchSize = 50
	}
	if err := generateMissing(ctx, client, cache, *cacheFlag, lemmas, batchSize, *dryRunFlag, out); err != nil {
		return err
	}
	if *dryRunFlag {
		return nil
	}
	return writeFixtures(*fixturesFlag, lemmas, cache, out)
}

// generateMissing asks the model for every lemma the cache does not hold, one
// batch at a time, saving the cache after each so a failure resumes here.
func generateMissing(
	ctx context.Context,
	client ai.Client,
	cache map[string]modelMeaning,
	cacheFile string,
	lemmas []string,
	batchSize int,
	dryRun bool,
	out io.Writer,
) error {
	for start := 0; start < len(lemmas); start += batchSize {
		end := min(start+batchSize, len(lemmas))
		batch := missingFromCache(lemmas[start:end], cache)
		if len(batch) == 0 {
			continue
		}
		if dryRun {
			_, _ = fmt.Fprintf(out, "Would ask the model for %d lemma(s): %s …\n",
				len(batch), strings.Join(batch, ", "))
			continue
		}
		meanings, genErr := generateMeanings(ctx, client, batch)
		if genErr != nil {
			// The cache is saved up to the last complete batch, so the next run
			// resumes here.
			if saveErr := writeCache(cacheFile, cache); saveErr != nil {
				return saveErr
			}
			return fmt.Errorf("generate meanings for %d lemma(s): %w", len(batch), genErr)
		}
		for _, meaning := range meanings {
			cache[strings.ToLower(meaning.Lemma)] = meaning
		}
		if err := writeCache(cacheFile, cache); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "  ✓ %d/%d lemma(s) written\n", min(end, len(lemmas)), len(lemmas))
	}
	return nil
}

// readSource reads the frequency list, keeping the first limit headwords that
// look like words: letters only, at least two characters, lower-cased and
// deduplicated. A proper noun or an abbreviation a real list carries is dropped
// here (WO 22 D22-6).
func readSource(path string, limit int) ([]string, error) {
	file, err := os.Open(path) //nolint:gosec // the operator's own source list
	if err != nil {
		return nil, fmt.Errorf("open source list: %w", err)
	}
	defer func() { _ = file.Close() }()

	seen := make(map[string]bool)
	var lemmas []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// `rank<TAB>lemma`, or just the lemma.
		if rank, lemma, found := strings.Cut(line, "\t"); found {
			if _, err := strconv.Atoi(rank); err == nil {
				line = lemma
			}
		}
		word := strings.ToLower(strings.TrimSpace(line))
		if !isWord(word) || seen[word] {
			continue
		}
		seen[word] = true
		lemmas = append(lemmas, word)
		if limit > 0 && len(lemmas) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read source list: %w", err)
	}
	return lemmas, nil
}

func isWord(word string) bool {
	if len(word) < 2 {
		return false
	}
	for _, r := range word {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return !profanity[word]
}

// profanity is dropped from the list: a learner's deck is not the place for it
// (WO 22 D22-6, "drop ... profanity"). Deliberately short — the common words a
// frequency list carries that a course would not teach.
var profanity = map[string]bool{
	"arse": true, "ass": true, "bastard": true, "bitch": true, "bollocks": true,
	"bugger": true, "cock": true, "crap": true, "cunt": true, "damn": true,
	"dick": true, "fuck": true, "fucking": true, "hell": true, "piss": true,
	"prick": true, "pussy": true, "shit": true, "slut": true, "twat": true,
	"whore": true,
}

// generateMeanings asks the model for one batch, fifty lemmas per call.
func generateMeanings(ctx context.Context, client ai.Client, lemmas []string) ([]modelMeaning, error) {
	response, err := client.Complete(ctx, ai.Request{
		Task: ai.TaskVocabMeanings,
		Vars: map[string]any{"Lemmas": strings.Join(lemmas, "\n")},
	})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Words []modelMeaning `json:"words"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(response.Text)), &payload); err != nil {
		// The reply is echoed on a parse failure: a model that changed its
		// shape is diagnosable from the run, not from a second run.
		preview := response.Text
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return nil, fmt.Errorf("parse meanings: %w (reply: %s)", err, preview)
	}
	return payload.Words, nil
}

// extractJSONObject returns the span from the first "{" to the last "}", so a
// reply that wrapped its JSON in prose still parses.
func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return text
	}
	return text[start : end+1]
}

// flaggedWord is a word the automatic checks refused, listed for a person
// (WO 22 D22-9).
type flaggedWord struct {
	Lemma  string `json:"lemma"`
	Reason string `json:"reason"`
}

// writeFixtures groups the meanings by CEFR level and writes one file per level.
// A word that fails the checks is flagged rather than written, and listed in
// flagged.json for a person.
func writeFixtures(dir string, lemmas []string, cache map[string]modelMeaning, out io.Writer) error {
	byLevel := map[string][]vocabfixture.Word{}
	var flagged []flaggedWord
	for _, lemma := range lemmas {
		meaning, ok := cache[lemma]
		if !ok {
			continue
		}
		level := clampLevel(meaning.CEFRLevel)
		word := vocabfixture.Word{
			Lemma:        meaning.Lemma,
			POS:          meaning.POS,
			CEFRLevel:    level,
			Definition:   meaning.Definition,
			DefinitionVI: meaning.DefinitionVI,
			Examples:     meaning.Examples,
		}
		if err := vocabfixture.ValidateWord(word, level); err != nil {
			flagged = append(flagged, flaggedWord{Lemma: lemma, Reason: err.Error()})
			continue
		}
		byLevel[level] = append(byLevel[level], word)
	}

	if len(flagged) > 0 {
		raw, err := json.MarshalIndent(flagged, "", "  ")
		if err != nil {
			return fmt.Errorf("encode flagged words: %w", err)
		}
		path := filepath.Join(dir, "flagged.json")
		if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
			return fmt.Errorf("write flagged words: %w", err)
		}
		_, _ = fmt.Fprintf(out, "  ! %d word(s) failed the checks, listed in %s\n", len(flagged), path)
	}

	levels := make([]string, 0, len(byLevel))
	for level := range byLevel {
		levels = append(levels, level)
	}
	sort.Strings(levels)

	for _, level := range levels {
		file := &vocabfixture.File{
			Source:    "wordfreq 3.1.1 (top_n_list, English)",
			Licence:   "wordfreq data: MIT package; frequency data from the listed open sources",
			CheckedAt: time.Now().UTC().Format("2006-01-02"),
			Level:     level,
			Words:     byLevel[level],
		}
		path, err := vocabfixture.WriteFile(dir, file)
		if err != nil {
			return err
		}
		// Read it back through the loader the seed uses: a file the seed would
		// refuse must not be written in the first place.
		if _, err := vocabfixture.LoadWords(path); err != nil {
			return fmt.Errorf("the written fixture %s is not loadable: %w", path, err)
		}
		_, _ = fmt.Fprintf(out, "  ✓ %s: %d word(s) -> %s\n", level, len(file.Words), path)
	}
	return nil
}

// clampLevel keeps the model's level inside A1–C1 and flags a disagreement by
// falling back to B1 (WO 22 Stage C trap 2).
func clampLevel(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "A1", "A2", "B1", "B2", "C1":
		return strings.ToUpper(strings.TrimSpace(level))
	case "C2":
		return "C1"
	default:
		return "B1"
	}
}

func missingFromCache(lemmas []string, cache map[string]modelMeaning) []string {
	var missing []string
	for _, lemma := range lemmas {
		if _, ok := cache[lemma]; !ok {
			missing = append(missing, lemma)
		}
	}
	return missing
}

// mergeCacheFromFixtures adds every word already frozen in dir to the cache, so
// a resumed run builds only what is missing. The level files are the durable
// record; the scratch cache is only a convenience.
func mergeCacheFromFixtures(dir string, cache map[string]modelMeaning) error {
	files, err := vocabfixture.ReadAll(dir)
	if err != nil {
		return fmt.Errorf("read the word fixtures: %w", err)
	}
	for _, file := range files {
		for _, word := range file.Words {
			lemma := strings.ToLower(word.Lemma)
			if _, ok := cache[lemma]; ok {
				continue
			}
			cache[lemma] = modelMeaning{
				Lemma:        word.Lemma,
				POS:          word.POS,
				CEFRLevel:    word.CEFRLevel,
				Definition:   word.Definition,
				DefinitionVI: word.DefinitionVI,
				Examples:     word.Examples,
			}
		}
	}
	return nil
}

func readCache(path string) (map[string]modelMeaning, error) {
	cache := map[string]modelMeaning{}
	raw, err := os.ReadFile(path) //nolint:gosec // the operator's own scratch file
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	return cache, nil
}

func writeCache(path string, cache map[string]modelMeaning) error {
	raw, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}

// loadConfig reads this command's configuration. Every key it may read is
// declared, because config.Load drops any key a command does not.
func loadConfig(ctx context.Context) (vocabCLIConfig, error) {
	var cfg vocabCLIConfig
	defaults := map[string]any{
		"app.environment":        "local",
		"ai.provider_1_name":     "",
		"ai.provider_1_base_url": "",
		"ai.provider_1_model":    "",
		"ai.provider_1_api_key":  "",
		"ai.provider_1_timeout":  120 * time.Second,
		"ai.provider_2_name":     "",
		"ai.provider_2_base_url": "",
		"ai.provider_2_model":    "",
		"ai.provider_2_api_key":  "",
		"ai.provider_2_timeout":  120 * time.Second,
		"ai.provider_3_name":     "",
		"ai.provider_3_base_url": "",
		"ai.provider_3_model":    "",
		"ai.provider_3_api_key":  "",
		"ai.provider_3_timeout":  120 * time.Second,
		"ai.provider_4_name":     "",
		"ai.provider_4_base_url": "",
		"ai.provider_4_model":    "",
		"ai.provider_4_api_key":  "",
		"ai.provider_4_timeout":  120 * time.Second,
		"ai.auto_publish":        false,
	}
	if err := config.Load(ctx, config.Options{
		Defaults: defaults,
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
		},
	}, &cfg); err != nil {
		return cfg, fmt.Errorf("load vocabgen configuration: %w", err)
	}
	return cfg, nil
}

func (c vocabCLIConfig) aiProviders() []ai.ProviderConfig {
	slots := []ai.ProviderConfig{
		{
			Name: c.AI.Provider1Name, BaseURL: c.AI.Provider1BaseURL, Model: c.AI.Provider1Model,
			APIKey: c.AI.Provider1APIKey, Timeout: c.AI.Provider1Timeout,
		},
		{
			Name: c.AI.Provider2Name, BaseURL: c.AI.Provider2BaseURL, Model: c.AI.Provider2Model,
			APIKey: c.AI.Provider2APIKey, Timeout: c.AI.Provider2Timeout,
		},
		{
			Name: c.AI.Provider3Name, BaseURL: c.AI.Provider3BaseURL, Model: c.AI.Provider3Model,
			APIKey: c.AI.Provider3APIKey, Timeout: c.AI.Provider3Timeout,
		},
		{
			Name: c.AI.Provider4Name, BaseURL: c.AI.Provider4BaseURL, Model: c.AI.Provider4Model,
			APIKey: c.AI.Provider4APIKey, Timeout: c.AI.Provider4Timeout,
		},
	}
	var live []ai.ProviderConfig
	for _, slot := range slots {
		if strings.TrimSpace(slot.Name) != "" && strings.TrimSpace(slot.APIKey) != "" {
			live = append(live, slot)
		}
	}
	return live
}

func initAIClient(ctx context.Context, cfg vocabCLIConfig, pool *pgxpool.Pool) ai.Client {
	if providers := cfg.aiProviders(); len(providers) > 0 {
		client, err := ai.New(ai.Config{Pool: pool, Providers: providers})
		if err == nil {
			return client
		}
		slog.WarnContext(ctx, "failed to initialize AI client, falling back to mock", "error", err)
	}
	registry, err := ai.NewRegistry()
	if err != nil {
		slog.WarnContext(ctx, "failed to load prompt registry", "error", err)
	}
	return ai.NewMockProvider(registry)
}
