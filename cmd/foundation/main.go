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

type foundationCLIConfig struct {
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
	} `koanf:"ai"`
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

	if err := flags.Parse(args); err != nil {
		return err
	}

	targetNode := strings.TrimSpace(*nodeFlag)
	if targetNode == "" && !*allFlag {
		return errors.New("must specify either -node CODE or -all")
	}

	var cfg foundationCLIConfig
	if err := config.Load(ctx, config.Options{
		Defaults: map[string]any{
			"app.environment": "local",
		},
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
		},
	}, &cfg); err != nil {
		return fmt.Errorf("load foundation configuration: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	nodes, err := querySpineNodes(ctx, pool, targetNode, *limitFlag)
	if err != nil {
		return fmt.Errorf("query spine nodes: %w", err)
	}

	if len(nodes) == 0 {
		if targetNode != "" {
			return fmt.Errorf("spine node %q not found", targetNode)
		}
		return errors.New("no spine taxonomy nodes found in database")
	}

	_, _ = fmt.Fprintf(out, "Found %d spine taxonomy node(s) to process.\n", len(nodes))

	if *dryRunFlag {
		for i, n := range nodes {
			kinds := exerciseKindsForNode(n.Namespace, n.Code)
			_, _ = fmt.Fprintf(out, "[dry-run] Node %d/%d: %s:%s (%s, %s)\n",
				i+1, len(nodes), n.Namespace, n.Code, n.Label, n.CEFRLevel)
			_, _ = fmt.Fprintf(out, "          - 1x foundation_topic\n")
			_, _ = fmt.Fprintf(out, "          - 3x exercises: %s\n", strings.Join(kinds, ", "))
			_, _ = fmt.Fprintf(out, "          - 1x foundation_quiz\n")
			_, _ = fmt.Fprintf(out, "          - 1x foundation_review\n")
		}
		return nil
	}

	generator, authorID, err := buildGenerator(ctx, cfg, pool)
	if err != nil {
		return fmt.Errorf("build generator: %w", err)
	}

	return generateDraftsForNodes(ctx, generator, nodes, authorID, out)
}

func querySpineNodes(ctx context.Context, pool *pgxpool.Pool, targetNode string, limit int) ([]spineNodeRow, error) {
	query := `
		SELECT id, namespace, code, label, COALESCE(cefr_level, 'B1') AS cefr_level
		FROM content.taxonomies
		WHERE namespace IN ('grammar', 'vocabulary', 'pattern', 'pronunciation', 'skill')
		  AND deprecated_at IS NULL
		  AND ($1 = '' OR code = $1)
		ORDER BY position ASC, code ASC`
	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := pool.Query(ctx, query, targetNode)
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
	authorID := resolveFirstAdminOrNew(ctx, pool)

	contentMod := content.New(content.Deps{
		Pool:  pool,
		Clock: clock.Real{},
	})

	lessonMod := lesson.New(lesson.Deps{
		Pool:    pool,
		Clock:   clock.Real{},
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
		Taxonomies:        contentMod.TaxonomyResolver(),
		Graders:           graders,
		AI:                aiClient,
		Clock:             clock.Real{},
		GeneratorAuthorID: authorID,
	})

	return learningMod.Generator(), authorID, nil
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

func resolveFirstAdminOrNew(ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	var adminID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT ur.user_id
		FROM rbac.user_roles ur
		JOIN rbac.roles r ON r.id = ur.role_id
		WHERE r.name = 'admin'
		ORDER BY ur.created_at ASC
		LIMIT 1`).Scan(&adminID)
	if err == nil && adminID != uuid.Nil {
		return adminID
	}
	return uuid.New()
}

func initAIClient(ctx context.Context, cfg foundationCLIConfig, pool *pgxpool.Pool) ai.Client {
	if cfg.AI.Provider1APIKey != "" {
		aiClient, err := ai.New(ai.Config{
			Pool: pool,
			Providers: []ai.ProviderConfig{
				{
					Name:    cfg.AI.Provider1Name,
					BaseURL: cfg.AI.Provider1BaseURL,
					Model:   cfg.AI.Provider1Model,
					APIKey:  cfg.AI.Provider1APIKey,
					Timeout: cfg.AI.Provider1Timeout,
				},
			},
		})
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
	out io.Writer,
) error {
	totalGenerated := 0
	for i, node := range nodes {
		_, _ = fmt.Fprintf(out, "[%d/%d] Generating drafts for node %s:%s (%s, level %s)...\n",
			i+1, len(nodes), node.Namespace, node.Code, node.Label, node.CEFRLevel)

		// 1. Foundation topic body
		topicReq := learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationTopic,
			CEFRLevel:  node.CEFRLevel,
			NodeCodes:  []string{node.Code},
			Purpose:    purposeFoundation,
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("foundation-topic-%s", strings.ToLower(node.Code)),
		}
		topicItems, err := generator.Generate(ctx, topicReq)
		if err != nil {
			return fmt.Errorf("generate foundation topic for %s: %w", node.Code, err)
		}
		totalGenerated += len(topicItems)
		_, _ = fmt.Fprintf(out, "  ✓ topic (version %s)\n", topicItems[0].ContentVersionID)

		// 2. Three namespace exercises
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
			}
			exItems, err := generator.Generate(ctx, exReq)
			if err != nil {
				return fmt.Errorf("generate exercise %d (%s) for %s: %w", exIdx+1, exKind, node.Code, err)
			}
			totalGenerated += len(exItems)
			_, _ = fmt.Fprintf(out, "  ✓ exercise %d: %s (version %s)\n", exIdx+1, exKind, exItems[0].ContentVersionID)
		}

		// 3. One foundation_quiz item
		quizReq := learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationQuiz,
			CEFRLevel:  node.CEFRLevel,
			NodeCodes:  []string{node.Code},
			Purpose:    purposeFoundation,
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("foundation-quiz-%s", strings.ToLower(node.Code)),
		}
		quizItems, err := generator.Generate(ctx, quizReq)
		if err != nil {
			return fmt.Errorf("generate quiz for %s: %w", node.Code, err)
		}
		totalGenerated += len(quizItems)
		_, _ = fmt.Fprintf(out, "  ✓ quiz (version %s)\n", quizItems[0].ContentVersionID)

		// 4. One foundation_review item
		reviewReq := learningcontract.GenerateRequest{
			Kind:       learningcontract.KindFoundationReview,
			CEFRLevel:  node.CEFRLevel,
			NodeCodes:  []string{node.Code},
			Purpose:    purposeFoundation,
			Count:      1,
			OwnerID:    &authorID,
			SlugPrefix: fmt.Sprintf("foundation-review-%s", strings.ToLower(node.Code)),
		}
		reviewItems, err := generator.Generate(ctx, reviewReq)
		if err != nil {
			return fmt.Errorf("generate review for %s: %w", node.Code, err)
		}
		totalGenerated += len(reviewItems)
		_, _ = fmt.Fprintf(out, "  ✓ review (version %s)\n", reviewItems[0].ContentVersionID)
	}

	_, _ = fmt.Fprintf(out, "Successfully generated %d draft items across %d spine nodes.\n",
		totalGenerated, len(nodes))
	return nil
}
