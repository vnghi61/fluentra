package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/genkit"
	"github.com/fluentra/fluentra/internal/modules/exam"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// extraRounds is how many rounds past -tests a run may take to make up for the
// groups the verifier doubted: each round generates one test's worth plus a
// margin, and a doubted group waits for a person instead of filling a test.
const extraRounds = 3

// generateTests is Stage N step 1: generate each part's questions a test's
// worth at a time, through the independent verifier, until the published bank
// composes `tests` disjoint fixed tests or the rounds run out. It reuses the
// daily job's generation (GenerateDailyExam), so the command and the job ask
// for the same shapes.
func generateTests(
	ctx context.Context, cfg examCLIConfig, pool *pgxpool.Pool, examCode string, tests int, dryRun bool,
	photos exam.PhotoSource, out io.Writer,
) error {
	have, err := fixedTestCount(ctx, pool, examCode)
	if err != nil {
		return err
	}
	if have >= tests {
		_, _ = fmt.Fprintf(out, "%s already has %d fixed test(s); nothing to generate.\n", examCode, have)
		return nil
	}
	if dryRun {
		_, _ = fmt.Fprintf(out, "Would generate %s toward %d fixed test(s) (has %d), at most %d round(s).\n",
			examCode, tests, have, tests-have+extraRounds)
		return nil
	}

	authorID, err := genkit.ResolveAdmin(ctx, pool)
	if err != nil {
		return err
	}
	examModule := assembleExam(pool, cfg.AI.client(ctx, pool), cfg.AI.AutoPublish, authorID, photos)

	for round := 1; have < tests && round <= tests-have+extraRounds; round++ {
		composed, err := examModule.Service().GenerateDailyExam(ctx, examCode)
		if err != nil {
			return fmt.Errorf("round %d: %w", round, err)
		}
		have, err = fixedTestCount(ctx, pool, examCode)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "  ✓ round %d: %d test(s) composed, %d fixed test(s) now\n", round, composed, have)
	}
	if have < tests {
		_, _ = fmt.Fprintf(out,
			"  ! %s has %d of %d fixed test(s): the rest wait for the doubted batches a person approves "+
				"(the review queue), then `cmd/examgen -exam %s -tests %d` again.\n",
			examCode, have, tests, examCode, tests)
	}
	_, _ = fmt.Fprintf(out, "Then freeze it: go run ./cmd/examgen -exam %s -export\n", examCode)
	return nil
}

// assembleExam wires the exam module with a question bank that generates
// through the shared generator, as the worker's daily job does, and the Part 1
// photographs it writes photo parts from.
func assembleExam(
	pool *pgxpool.Pool, client ai.Client, autoPublish bool, authorID uuid.UUID, photos exam.PhotoSource,
) *exam.Module {
	kit := genkit.Assemble(pool, client, autoPublish, authorID)
	var examModule *exam.Module
	bank := questionbank.New(questionbank.Deps{
		Pool:          pool,
		ContentReader: kit.Content.Reader(),
		TagIndex:      kit.Content.TagIndex(),
		LessonAuthor:  kit.Lesson.Author(),
		Generator:     kit.Generator(),
		ExamParts:     lazyExamParts{of: &examModule},
	})
	examModule = exam.New(exam.Deps{
		Pool:         pool,
		Lesson:       kit.Lesson.Reader(),
		Questionbank: bank.Reader(),
		BankAuthor:   bank.Author(),
		Photos:       photos,
	})
	return examModule
}

// lazyExamParts answers the bank's exam-part lookup once the exam module
// exists: the bank is built first so the exam module can generate through it.
type lazyExamParts struct{ of **exam.Module }

func (l lazyExamParts) PartConstraints(
	ctx context.Context, partID uuid.UUID,
) (*learningcontract.ExamPartConstraints, error) {
	if l.of == nil || *l.of == nil {
		return nil, nil
	}
	return (*l.of).Service().PartConstraints(ctx, partID)
}

// fixedTestCount is how many numbered fixed tests the exam's blueprints hold.
func fixedTestCount(ctx context.Context, pool *pgxpool.Pool, examCode string) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM assess.mock_tests m
		JOIN assess.blueprints b ON b.id = m.blueprint_id
		JOIN assess.exam_versions v ON v.id = b.version_id
		WHERE v.code = $1 AND m.mode = 'fixed'`, examCode).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count fixed tests of %s: %w", examCode, err)
	}
	return count, nil
}

// aiConfig is the four provider slots and the auto-publish switch, as the API,
// the worker and cmd/foundation read them.
type aiConfig struct {
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
	// AutoPublish lets the independent verifier publish what it confirms.
	AutoPublish bool `koanf:"auto_publish"`
}

// defaultAITimeout matches the API and the worker.
const defaultAITimeout = 120 * time.Second

// aiDefaults declares every ai.* key, because config.Load drops any key a
// command does not declare.
func aiDefaults() map[string]any {
	defaults := map[string]any{"ai.auto_publish": false}
	for slot := 1; slot <= 4; slot++ {
		prefix := fmt.Sprintf("ai.provider_%d_", slot)
		defaults[prefix+"name"] = ""
		defaults[prefix+"base_url"] = ""
		defaults[prefix+"model"] = ""
		defaults[prefix+"api_key"] = ""
		defaults[prefix+"timeout"] = defaultAITimeout
	}
	return defaults
}

func (c aiConfig) providers() []ai.ProviderConfig {
	slots := []ai.ProviderConfig{
		{Name: c.Provider1Name, BaseURL: c.Provider1BaseURL, Model: c.Provider1Model,
			APIKey: c.Provider1APIKey, Timeout: c.Provider1Timeout},
		{Name: c.Provider2Name, BaseURL: c.Provider2BaseURL, Model: c.Provider2Model,
			APIKey: c.Provider2APIKey, Timeout: c.Provider2Timeout},
		{Name: c.Provider3Name, BaseURL: c.Provider3BaseURL, Model: c.Provider3Model,
			APIKey: c.Provider3APIKey, Timeout: c.Provider3Timeout},
		{Name: c.Provider4Name, BaseURL: c.Provider4BaseURL, Model: c.Provider4Model,
			APIKey: c.Provider4APIKey, Timeout: c.Provider4Timeout},
	}
	var live []ai.ProviderConfig
	for _, slot := range slots {
		if strings.TrimSpace(slot.Name) != "" && strings.TrimSpace(slot.APIKey) != "" {
			live = append(live, slot)
		}
	}
	return live
}

// client is the configured AI client, or the offline mock when none is.
func (c aiConfig) client(ctx context.Context, pool *pgxpool.Pool) ai.Client {
	if providers := c.providers(); len(providers) > 0 {
		client, err := ai.New(ai.Config{Pool: pool, Providers: providers})
		if err == nil {
			return client
		}
		slog.WarnContext(ctx, "failed to initialise the AI client, falling back to the mock", "error", err)
	}
	registry, err := ai.NewRegistry()
	if err != nil {
		slog.WarnContext(ctx, "failed to load the prompt registry", "error", err)
	}
	return ai.NewMockProvider(registry)
}
