// Command foundation generates draft content sets for foundation spine taxonomy nodes.
//
// Conforms to Work Order 19 §D:
//  1. cmd/foundation -node CODE | -all calls Generator with purpose "foundation":
//     - one foundation_topic body through task foundation_topic_generate
//     - three exercises of kinds suited to the node's namespace
//     - one foundation_quiz and one foundation_review item
//     All land as drafts, tagged to the node in content.content_tags.
//  2. Publishing the topic is what enforces completeness (BR-FOUNDATION-05).
//  3. foundation_quiz and foundation_review are registered as multiple-choice grader aliases.
//  4. Each node's cefr_level is set when its topic is approved.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/content"
	"github.com/fluentra/fluentra/internal/modules/grammar"
	grammarcontract "github.com/fluentra/fluentra/internal/modules/grammar/contract"
	"github.com/fluentra/fluentra/internal/modules/learning"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/modules/listening"
	listeningcontract "github.com/fluentra/fluentra/internal/modules/listening/contract"
	"github.com/fluentra/fluentra/internal/modules/reading"
	readingcontract "github.com/fluentra/fluentra/internal/modules/reading/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking"
	speakingcontract "github.com/fluentra/fluentra/internal/modules/speaking/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary"
	vocabularycontract "github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/modules/writing"
	writingcontract "github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/config"
)

const (
	kindGrammarTenseChoice       = "grammar_tense_choice"
	kindGrammarSentenceTransform = "grammar_sentence_transform"
	kindSpeakingTask             = "speaking_task"
	kindListeningComprehension   = "listening_comprehension"
	kindReadingComprehension     = "reading_comprehension"
	nsSkill                      = "skill"
	purposeFoundation            = "foundation"
)

// defaultAITimeout matches the API and the worker.
const defaultAITimeout = 120 * time.Second

type foundationCLIConfig struct {
	App struct {
		Environment string `koanf:"environment"`
	} `koanf:"app"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	// All four provider slots, as the API and the worker read them. Reading
	// only the first meant a deployment that had put its working key in slot 2
	// generated with none at all.
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
		// AutoPublish publishes a node's items the independent verifier confirms
		// (WO 22 Stage A).
		AutoPublish bool `koanf:"auto_publish"`
	} `koanf:"ai"`
}

// defaultFixtureDir is where an -export writes the frozen Foundation content
// the seed loads offline (WO 22 Stage G).
const defaultFixtureDir = "db/fixtures/foundation"

// printFoundationDryRun lists what a real run would generate for each node.
func printFoundationDryRun(nodes []spineNodeRow, out io.Writer) {
	for i, node := range nodes {
		kinds := exerciseKindsForNode(node.Namespace, node.Code)
		_, _ = fmt.Fprintf(out, "[dry-run] Node %d/%d: %s:%s (%s, %s)\n",
			i+1, len(nodes), node.Namespace, node.Code, node.Label, node.CEFRLevel)
		_, _ = fmt.Fprintf(out, "          - 1x foundation_topic\n")
		_, _ = fmt.Fprintf(out, "          - 3x exercises: %s\n", strings.Join(kinds, ", "))
		_, _ = fmt.Fprintf(out, "          - 1x foundation_quiz\n")
		_, _ = fmt.Fprintf(out, "          - 1x foundation_review\n")
	}
}

type spineNodeRow struct {
	ID        uuid.UUID
	Namespace string
	Code      string
	Label     string
	CEFRLevel string
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "foundation command error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("foundation", flag.ContinueOnError)
	nodeFlag := flags.String("node", "", "Specific spine taxonomy node code to generate drafts for (e.g. PRESENT_PERFECT)")
	allFlag := flags.Bool("all", false, "Generate drafts for all spine taxonomy nodes")
	limitFlag := flags.Int("limit", 0, "Optional limit on number of nodes to process")
	dryRunFlag := flags.Bool("dry-run", false, "Simulate generation without persisting items")
	mockFlag := flags.Bool("mock", false, "Generate with the offline mock provider, writing placeholder drafts")
	exportFlag := flags.Bool("export", false, "Export published Foundation content to fixtures and exit")
	fixturesFlag := flags.String("fixtures", defaultFixtureDir,
		"Directory an -export writes to and `cmd/seed -foundation` reads")
	missingFlag := flags.Bool("missing", false,
		"Only nodes with no published topic yet, so a long run resumes in chunks")
	afterFlag := flags.String("after", "",
		"Resume after this node code, so a chunked run advances past a node that keeps failing")
	draftsFlag := flags.Bool("drafts", false,
		"With -missing, treat a node that has any topic version (draft too) as done, for a drafts-only run a person reviews later")

	if err := flags.Parse(args); err != nil {
		return err
	}

	targetNode := strings.TrimSpace(*nodeFlag)
	if targetNode == "" && !*allFlag && !*exportFlag && !*missingFlag {
		return errors.New("must specify either -node CODE or -all (or -export, or -missing)")
	}

	cfg, err := loadFoundationConfig(ctx)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	nodes, err := querySpineNodes(ctx, pool, targetNode, strings.TrimSpace(*afterFlag), *limitFlag, *missingFlag, *draftsFlag)
	if err != nil {
		return fmt.Errorf("query spine nodes: %w", err)
	}

	if len(nodes) == 0 {
		if targetNode != "" {
			return fmt.Errorf("spine node %q not found", targetNode)
		}
		if *missingFlag {
			_, _ = fmt.Fprintln(out, "Every spine node already has a published Foundation topic.")
			return nil
		}
		return errors.New("no spine taxonomy nodes found in database")
	}

	_, _ = fmt.Fprintf(out, "Found %d spine taxonomy node(s) to process.\n", len(nodes))

	if *exportFlag {
		_, _ = fmt.Fprintf(out, "Exporting published Foundation content to %s...\n", *fixturesFlag)
		return exportFoundationFixtures(ctx, pool, nodes, *fixturesFlag, out)
	}

	if *dryRunFlag {
		printFoundationDryRun(nodes, out)
		return nil
	}

	// A run with no provider writes the mock generator's output — six items per
	// node, indistinguishable from real drafts once they are in the review
	// queue. That is a worse outcome than not running, so it is refused rather
	// than warned about. -mock is for trying the plumbing on a throwaway
	// database.
	if len(cfg.aiProviders()) == 0 && !*mockFlag {
		return errors.New(
			"no AI provider is configured, so this would fill the database with mock drafts: " +
				"set AI_PROVIDER_1_NAME and AI_PROVIDER_1_API_KEY " +
				"(see .env.example), or pass -mock to do it anyway")
	}

	generator, authorID, err := buildGenerator(ctx, cfg, pool)
	if err != nil {
		return fmt.Errorf("build generator: %w", err)
	}

	// One batch id per run, so a node's doubts are reviewed together and can be
	// told apart from another run's (WO 22 Stage A.4).
	runBatch := time.Now().UTC().Format("20060102T150405")
	return generateDraftsForNodes(ctx, generator, nodes, authorID, runBatch, out)
}

// loadFoundationConfig reads this command's configuration.
//
// A function of its own so a test can assert what actually reaches the
// struct: the keys declared here are the only ones config.Load will read
// from the environment, which is a rule that is easy to break silently.
func loadFoundationConfig(ctx context.Context) (foundationCLIConfig, error) {
	var cfg foundationCLIConfig
	if err := config.Load(ctx, config.Options{
		// Every ai.* key is declared, not merely read into the struct. A section
		// that appears in neither Defaults nor Required is dropped from the
		// environment entirely — so before this, AI_PROVIDER_1_API_KEY never
		// reached the struct, initAIClient saw no key, and the command would
		// have written a database full of mock drafts believing they were real.
		Defaults: map[string]any{
			"app.environment":        "local",
			"ai.provider_1_name":     "",
			"ai.provider_1_base_url": "",
			"ai.provider_1_model":    "",
			"ai.provider_1_api_key":  "",
			"ai.provider_1_timeout":  defaultAITimeout,
			"ai.provider_2_name":     "",
			"ai.provider_2_base_url": "",
			"ai.provider_2_model":    "",
			"ai.provider_2_api_key":  "",
			"ai.provider_2_timeout":  defaultAITimeout,
			"ai.provider_3_name":     "",
			"ai.provider_3_base_url": "",
			"ai.provider_3_model":    "",
			"ai.provider_3_api_key":  "",
			"ai.provider_3_timeout":  defaultAITimeout,
			"ai.provider_4_name":     "",
			"ai.provider_4_base_url": "",
			"ai.provider_4_model":    "",
			"ai.provider_4_api_key":  "",
			"ai.provider_4_timeout":  defaultAITimeout,
			"ai.auto_publish":        false,
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
		},
	}, &cfg); err != nil {
		return cfg, fmt.Errorf("load foundation configuration: %w", err)
	}
	return cfg, nil
}

func querySpineNodes(
	ctx context.Context, pool *pgxpool.Pool,
	targetNode, after string, limit int, missingOnly, includeDrafts bool,
) ([]spineNodeRow, error) {
	// `missingOnly` makes a long generation resumable: the topic is the last
	// item a node publishes, so "no published foundation_topic tagged to the
	// node" is the completion signal, and a chunked run advances.
	query := `
		SELECT t.id, t.namespace, t.code, t.label, COALESCE(t.cefr_level, 'B1') AS cefr_level
		FROM content.taxonomies t
		WHERE t.namespace IN ('grammar', 'vocabulary', 'pattern', 'pronunciation', 'skill')
		  AND t.deprecated_at IS NULL
		  AND ($1 = '' OR t.code = $1)
		  AND ($3 = '' OR t.code > $3)
		  AND (NOT $2::boolean OR NOT EXISTS (
		      SELECT 1
		      FROM content.content_tags ct
		      JOIN content.content_items i ON i.id = ct.item_id
		      JOIN content.content_versions v ON v.id = i.current_version_id
		      WHERE ct.taxonomy_id = t.id
		        AND i.status = 'published' AND v.status = 'published'
		        AND v.kind = 'foundation_topic'
		  ))
		ORDER BY t.code ASC`
	if missingOnly && includeDrafts {
		// A drafts-only run treats any topic version as done, so it advances
		// without needing publication (a person reviews the drafts later).
		query = strings.Replace(query,
			"AND i.status = 'published' AND v.status = 'published'",
			"AND (v.status = 'published' OR v.status = 'draft')", 1)
	}
	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := pool.Query(ctx, query, targetNode, missingOnly, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []spineNodeRow
	for rows.Next() {
		var r spineNodeRow
		if err := rows.Scan(&r.ID, &r.Namespace, &r.Code, &r.Label, &r.CEFRLevel); err != nil {
			return nil, err
		}
		nodes = append(nodes, r)
	}
	return nodes, rows.Err()
}

func exerciseKindsForNode(namespace, code string) []string {
	switch namespace {
	case "grammar":
		return []string{
			kindGrammarTenseChoice,
			kindGrammarSentenceTransform,
			kindGrammarTenseChoice,
		}
	case "vocabulary":
		return []string{
			"vocab_multiple_choice",
			kindGrammarTenseChoice,
			kindGrammarSentenceTransform,
		}
	case "pattern":
		return []string{
			kindGrammarSentenceTransform,
			kindGrammarTenseChoice,
			kindGrammarSentenceTransform,
		}
	case "pronunciation":
		return []string{
			kindGrammarTenseChoice,
			kindSpeakingTask,
			kindListeningComprehension,
		}
	case nsSkill:
		switch code {
		case "READING":
			return []string{kindReadingComprehension, kindReadingComprehension, kindGrammarTenseChoice}
		case "LISTENING":
			return []string{kindListeningComprehension, kindListeningComprehension, kindGrammarTenseChoice}
		case "WRITING":
			return []string{"writing_prompt", kindGrammarSentenceTransform, kindGrammarTenseChoice}
		case "SPEAKING":
			return []string{kindSpeakingTask, kindSpeakingTask, kindGrammarTenseChoice}
		default:
			return []string{kindGrammarTenseChoice, kindGrammarSentenceTransform, kindReadingComprehension}
		}
	default:
		return []string{
			kindGrammarTenseChoice,
			kindGrammarSentenceTransform,
			kindGrammarTenseChoice,
		}
	}
}

func buildGenerator(
	ctx context.Context,
	cfg foundationCLIConfig,
	pool *pgxpool.Pool,
) (learningcontract.Generator, uuid.UUID, error) {
	authorID, err := resolveAuthorID(ctx, pool)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return assembleGenerator(ctx, cfg, pool, authorID), authorID, nil
}

// assembleGenerator is the module wiring alone, with no database read in it.
//
// Separate from buildGenerator so a test can assemble it without a database and
// catch a constructor that fails closed. This command panicked on boot for
// exactly that reason — content.New refuses a nil guard — and nothing noticed,
// because assembly only ever ran against a live database with an operator
// watching.
func assembleGenerator(
	ctx context.Context,
	cfg foundationCLIConfig,
	pool *pgxpool.Pool,
	authorID uuid.UUID,
) learningcontract.Generator {
	// NewAuthoring, not New: this is a CLI that mounts no routes, so it has no
	// guard to give and content.New fails closed without one. The worker learned
	// the same lesson — see content.NewAuthoring's comment.
	contentMod := content.NewAuthoring(content.Deps{
		Pool:  pool,
		Clock: clock.Real{},
	})

	lessonMod := lesson.New(lesson.Deps{
		Pool:  pool,
		Clock: clock.Real{},
		// lesson has no authoring-only constructor, so it takes the permissive
		// guard the worker gives it. Nothing here serves a request: the only
		// caller is this command, and it mounts no router to protect.
		Guard:   offlineGuard{},
		Content: contentMod.Reader(),
	})

	vocabMod := vocabulary.New(vocabulary.Deps{
		Pool:    pool,
		Content: contentMod.Reader(),
		Clock:   clock.Real{},
	})

	grammarMod := grammar.New(grammar.Deps{
		Content: contentMod.Reader(),
	})

	readingMod := reading.New(reading.Deps{
		Content: contentMod.Reader(),
	})

	listeningMod := listening.New(listening.Deps{
		Content: contentMod.Reader(),
	})

	writingMod := writing.New(writing.Deps{
		Content: contentMod.Reader(),
	})

	speakingMod := speaking.New(speaking.Deps{
		Content: contentMod.Reader(),
	})

	graders := assembleGraders(vocabMod, grammarMod, readingMod, listeningMod, writingMod, speakingMod)

	aiClient := initAIClient(ctx, cfg, pool)

	learningMod := learning.New(learning.Deps{
		Pool:              pool,
		Lesson:            lessonMod.Reader(),
		LessonAuthor:      lessonMod.Author(),
		Content:           contentMod.Reader(),
		ContentAuthor:     contentMod.Author(),
		ContentRecorder:   contentMod.VerificationRecorder(),
		AutoPublish:       cfg.AI.AutoPublish,
		Taxonomies:        contentMod.TaxonomyResolver(),
		Graders:           graders,
		AI:                aiClient,
		Clock:             clock.Real{},
		GeneratorAuthorID: authorID,
	})

	return learningMod.Generator()
}

func assembleGraders(
	vocabMod *vocabulary.Module,
	grammarMod *grammar.Module,
	readingMod *reading.Module,
	listeningMod *listening.Module,
	writingMod *writing.Module,
	speakingMod *speaking.Module,
) map[string]learningcontract.ExerciseGrader {
	graders := make(map[string]learningcontract.ExerciseGrader)
	for _, kind := range vocabularycontract.GradedKinds() {
		graders[kind] = vocabMod.Grader()
	}
	for _, kind := range grammarcontract.GradedKinds() {
		graders[kind] = grammarMod.Grader()
	}
	for _, kind := range readingcontract.GradedKinds() {
		graders[kind] = readingMod.Grader()
	}
	for _, kind := range listeningcontract.GradedKinds() {
		graders[kind] = listeningMod.Grader()
	}
	for _, kind := range writingcontract.GradedKinds() {
		graders[kind] = writingMod.Grader()
	}
	for _, kind := range speakingcontract.GradedKinds() {
		graders[kind] = speakingMod.Grader()
	}
	// Stage D: Register multiple-choice grader aliases for foundation_quiz and foundation_review
	graders[learningcontract.KindFoundationQuiz] = grammarMod.Grader()
	graders[learningcontract.KindFoundationReview] = grammarMod.Grader()
	return graders
}

// resolveAuthorID is the administrator every generated draft is attributed to.
//
// It used to read rbac.user_roles ordered by created_at. There is no rbac
// schema — the tables are core.roles and core.user_roles, and the column is
// granted_at — so the query always failed, and the fallback invented a fresh
// uuid for a user that does not exist. content_items.owner_id references
// core.users, so the first draft died on the foreign key, one AI call after the
// money was spent. An author who cannot be found is now an error, because a
// generated draft with no real owner cannot be stored at all.
func resolveAuthorID(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	var adminID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT ur.user_id
		FROM core.user_roles ur
		JOIN core.roles r ON r.id = ur.role_id
		WHERE r.name = 'admin'
		ORDER BY ur.granted_at ASC
		LIMIT 1`).Scan(&adminID)
	if err != nil {
		return uuid.Nil, fmt.Errorf(
			"find an admin to attribute the drafts to (run `make seed` first): %w", err)
	}
	if adminID == uuid.Nil {
		return uuid.Nil, errors.New(
			"no account holds the admin role, so generated drafts would have no owner; run `make seed` first")
	}
	return adminID, nil
}

// aiProviders is every configured provider slot, in order, as the API and the
// worker build it. A slot with no name is not a provider.
func (cfg foundationCLIConfig) aiProviders() []ai.ProviderConfig {
	slots := []ai.ProviderConfig{
		{
			Name: cfg.AI.Provider1Name, BaseURL: cfg.AI.Provider1BaseURL, Model: cfg.AI.Provider1Model,
			APIKey: cfg.AI.Provider1APIKey, Timeout: cfg.AI.Provider1Timeout,
		},
		{
			Name: cfg.AI.Provider2Name, BaseURL: cfg.AI.Provider2BaseURL, Model: cfg.AI.Provider2Model,
			APIKey: cfg.AI.Provider2APIKey, Timeout: cfg.AI.Provider2Timeout,
		},
		{
			Name: cfg.AI.Provider3Name, BaseURL: cfg.AI.Provider3BaseURL, Model: cfg.AI.Provider3Model,
			APIKey: cfg.AI.Provider3APIKey, Timeout: cfg.AI.Provider3Timeout,
		},
		{
			Name: cfg.AI.Provider4Name, BaseURL: cfg.AI.Provider4BaseURL, Model: cfg.AI.Provider4Model,
			APIKey: cfg.AI.Provider4APIKey, Timeout: cfg.AI.Provider4Timeout,
		},
	}
	live := make([]ai.ProviderConfig, 0, len(slots))
	for _, slot := range slots {
		if strings.TrimSpace(slot.Name) != "" && strings.TrimSpace(slot.APIKey) != "" {
			live = append(live, slot)
		}
	}
	return live
}

func initAIClient(ctx context.Context, cfg foundationCLIConfig, pool *pgxpool.Pool) ai.Client {
	if providers := cfg.aiProviders(); len(providers) > 0 {
		aiClient, err := ai.New(ai.Config{Pool: pool, Providers: providers})
		if err == nil {
			return aiClient
		}
		slog.WarnContext(ctx, "failed to initialize AI client with provider, falling back to mock", "error", err)
	}

	registry, err := ai.NewRegistry()
	if err != nil {
		slog.WarnContext(ctx, "failed to load prompt registry", "error", err)
	}
	return ai.NewMockProvider(registry)
}

func generateDraftsForNodes(
	ctx context.Context,
	generator learningcontract.Generator,
	nodes []spineNodeRow,
	authorID uuid.UUID,
	runBatch string,
	out io.Writer,
) error {
	totalGenerated := 0
	// One bad node must not stop a 93-node run: its failure is reported, the
	// rest continue, and `-missing` picks the failed node up on the next run.
	var failed []string
nodeLoop:
	for i, node := range nodes {
		_, _ = fmt.Fprintf(out, "[%d/%d] Generating drafts for node %s:%s (%s, level %s)...\n",
			i+1, len(nodes), node.Namespace, node.Code, node.Label, node.CEFRLevel)

		batch := fmt.Sprintf("foundation:%s:%s", node.Code, runBatch)

		// The exercises, quiz and review come first: a topic does not publish
		// until its node carries them (BR-FOUNDATION-05), and with auto-publish
		// on the topic is published in the same run.
		// 1. Three namespace exercises
		exKinds := exerciseKindsForNode(node.Namespace, node.Code)
		for exIdx, exKind := range exKinds {
			exSlug := fmt.Sprintf(
				"foundation-ex-%s-%s-%d",
				strings.ToLower(node.Code),
				strings.ReplaceAll(exKind, "_", "-"),
				exIdx+1,
			)
			exReq := learningcontract.GenerateRequest{
				Kind:       exKind,
				CEFRLevel:  node.CEFRLevel,
				NodeCodes:  []string{node.Code},
				Purpose:    purposeFoundation,
				Count:      1,
				OwnerID:    &authorID,
				SlugPrefix: exSlug,
				Batch:      batch,
			}
			exItems, err := generator.Generate(ctx, exReq)
			if err != nil {
				_, _ = fmt.Fprintf(out, "  ✗ exercise %d: %v\n", exIdx+1, err)
				failed = append(failed, node.Code)
				continue nodeLoop
			}
			totalGenerated += len(exItems)
			_, _ = fmt.Fprintf(out, "  ✓ exercise %d: %s (version %s)\n", exIdx+1, exKind, exItems[0].ContentVersionID)
		}

		// 2. One foundation_quiz item
		quizReq := learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationQuiz,
			CEFRLevel:  node.CEFRLevel,
			NodeCodes:  []string{node.Code},
			Purpose:    purposeFoundation,
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("foundation-quiz-%s", strings.ToLower(node.Code)),
			Batch:      batch,
		}
		quizItems, err := generator.Generate(ctx, quizReq)
		if err != nil {
			_, _ = fmt.Fprintf(out, "  ✗ quiz: %v\n", err)
			failed = append(failed, node.Code)
			continue nodeLoop
		}
		totalGenerated += len(quizItems)
		_, _ = fmt.Fprintf(out, "  ✓ quiz (version %s)\n", quizItems[0].ContentVersionID)

		// 3. One foundation_review item
		reviewReq := learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationReview,
			CEFRLevel:  node.CEFRLevel,
			NodeCodes:  []string{node.Code},
			Purpose:    purposeFoundation,
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("foundation-review-%s", strings.ToLower(node.Code)),
			Batch:      batch,
		}
		reviewItems, err := generator.Generate(ctx, reviewReq)
		if err != nil {
			_, _ = fmt.Fprintf(out, "  ✗ review: %v\n", err)
			failed = append(failed, node.Code)
			continue nodeLoop
		}
		totalGenerated += len(reviewItems)
		_, _ = fmt.Fprintf(out, "  ✓ review (version %s)\n", reviewItems[0].ContentVersionID)

		// 4. The topic itself, last: it publishes only once its exercises,
		// quiz and review are published.
		topicReq := learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationTopic,
			CEFRLevel:  node.CEFRLevel,
			NodeCodes:  []string{node.Code},
			Purpose:    purposeFoundation,
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("foundation-topic-%s", strings.ToLower(node.Code)),
			Batch:      batch,
		}
		topicItems, err := generator.Generate(ctx, topicReq)
		if err != nil {
			_, _ = fmt.Fprintf(out, "  ✗ topic: %v\n", err)
			failed = append(failed, node.Code)
			continue nodeLoop
		}
		totalGenerated += len(topicItems)
		_, _ = fmt.Fprintf(out, "  ✓ topic (version %s)\n", topicItems[0].ContentVersionID)
	}

	_, _ = fmt.Fprintf(out, "Generated %d item(s) across %d spine nodes.\n",
		totalGenerated, len(nodes))
	if len(failed) > 0 {
		return fmt.Errorf("%d node(s) failed and will be retried by -missing: %s",
			len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// offlineGuard permits everything, because there is nothing to protect: this
// command mounts no routes and runs as an operator who already has a shell on
// the machine. It exists only because lesson's constructor fails closed on a
// nil guard, which is the right default for a process that does serve HTTP.
type offlineGuard struct{}

func (offlineGuard) Require(_ context.Context, _ string) error { return nil }
