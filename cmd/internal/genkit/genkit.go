// Package genkit assembles the content generator the offline generation
// commands share: cmd/foundation (WO 22 Stage G) and cmd/examgen (Stage N).
// Both write through the same learning generator, independent verifier and
// content state machine as the API and the worker, so a frozen fixture holds
// what the product would have produced.
package genkit

import (
	"context"
	"errors"
	"fmt"

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
	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// Kit is the assembled modules a generation command writes through.
type Kit struct {
	Content  *content.Module
	Lesson   *lesson.Module
	Learning *learning.Module
}

// Generator is the learning generator every item is authored through.
func (k *Kit) Generator() learningcontract.Generator { return k.Learning.Generator() }

// Assemble wires the modules with no database read, so a test can assemble
// them without one and catch a constructor that fails closed. This command
// family once panicked on boot for exactly that reason: content.New refuses a
// nil guard, and a CLI mounts no routes to guard.
func Assemble(pool *pgxpool.Pool, client ai.Client, autoPublish bool, authorID uuid.UUID) *Kit {
	// NewAuthoring, not New: a CLI mounts no routes, so it has no guard to give.
	contentMod := content.NewAuthoring(content.Deps{Pool: pool, Clock: clock.Real{}})
	lessonMod := lesson.New(lesson.Deps{
		Pool:  pool,
		Clock: clock.Real{},
		// lesson has no authoring-only constructor, so it takes a permissive
		// guard: nothing here serves a request.
		Guard:   OfflineGuard{},
		Content: contentMod.Reader(),
	})
	vocabMod := vocabulary.New(vocabulary.Deps{Pool: pool, Content: contentMod.Reader(), Clock: clock.Real{}})
	grammarMod := grammar.New(grammar.Deps{Content: contentMod.Reader()})
	readingMod := reading.New(reading.Deps{Content: contentMod.Reader()})
	listeningMod := listening.New(listening.Deps{Content: contentMod.Reader()})
	writingMod := writing.New(writing.Deps{Content: contentMod.Reader()})
	speakingMod := speaking.New(speaking.Deps{Content: contentMod.Reader()})

	learningMod := learning.New(learning.Deps{
		Pool:              pool,
		Lesson:            lessonMod.Reader(),
		LessonAuthor:      lessonMod.Author(),
		Content:           contentMod.Reader(),
		ContentAuthor:     contentMod.Author(),
		ContentRecorder:   contentMod.VerificationRecorder(),
		AutoPublish:       autoPublish,
		Taxonomies:        contentMod.TaxonomyResolver(),
		Graders:           graders(vocabMod, grammarMod, readingMod, listeningMod, writingMod, speakingMod),
		AI:                client,
		Clock:             clock.Real{},
		GeneratorAuthorID: authorID,
		Charts:            media.ChartDataURI{},
	})
	return &Kit{Content: contentMod, Lesson: lessonMod, Learning: learningMod}
}

func graders(
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

// ResolveAdmin is the administrator every generated draft is attributed to.
//
// It used to read rbac.user_roles ordered by created_at. There is no rbac
// schema — the tables are core.roles and core.user_roles, and the column is
// granted_at — so the query always failed, and the fallback invented a fresh
// uuid for a user that does not exist. content_items.owner_id references
// core.users, so the first draft died on the foreign key, one AI call after the
// money was spent. An author who cannot be found is now an error, because a
// generated draft with no real owner cannot be stored at all.
func ResolveAdmin(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
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

// OfflineGuard permits everything, because there is nothing to protect: a
// generation command mounts no routes and runs as an operator with a shell.
type OfflineGuard struct{}

// Require permits every permission.
func (OfflineGuard) Require(_ context.Context, _ string) error { return nil }
