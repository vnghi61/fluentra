package main

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/examfixture"
	"github.com/fluentra/fluentra/internal/modules/content"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/exam"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/modules/questionbank"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	questionbankdomain "github.com/fluentra/fluentra/internal/modules/questionbank/domain"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

// defaultExamFixtureDir is where `cmd/examgen -export` writes the frozen exam
// questions this loader reads (WO 22 Stage N).
const defaultExamFixtureDir = "db/fixtures/exams"

// seedExamFixtures loads the frozen exam questions: it authors each content
// version with the approval it already had, puts it in the bank, and composes
// the numbered fixed tests the published bank can fill. No model call, so
// `make seed` is fast, offline and the same on every machine.
func seedExamFixtures(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, dir string, out io.Writer,
) error {
	files, err := examfixture.ReadAll(dir)
	if err != nil {
		return fmt.Errorf("read exam fixtures: %w", err)
	}
	if len(files) == 0 {
		_, _ = fmt.Fprintf(out,
			"No exam fixtures in %s; the exam hub will offer no tests until "+
				"`cmd/examgen -export` has run.\n", dir)
		return nil
	}

	contentMod := content.NewAuthoring(content.Deps{Pool: pool, Clock: clock.Real{}})
	author := contentMod.Author()
	recorder := contentMod.VerificationRecorder()

	lessonMod := lesson.New(lesson.Deps{
		Pool:    pool,
		Clock:   clock.Real{},
		Guard:   offlineGuard{},
		Content: contentMod.Reader(),
	})
	questionbankMod := questionbank.New(questionbank.Deps{
		Pool:          pool,
		RBAC:          offlineRBAC{},
		ContentReader: contentMod.Reader(),
		TagIndex:      contentMod.TagIndex(),
		LessonAuthor:  lessonMod.Author(),
		Events:        eventbus.NewInProcessBus(nil),
	})
	examMod := exam.New(exam.Deps{Pool: pool, Questionbank: questionbankMod.Reader()})

	published, drafted, questions := 0, 0, 0
	for _, file := range files {
		for _, item := range file.Items {
			versionID, itemPublished, err := authorExamItem(ctx, pool, author, recorder, adminID, item)
			if err != nil {
				return fmt.Errorf("author %s: %w", item.Slug, err)
			}
			if !itemPublished {
				drafted++
				continue
			}
			published++

			created, err := ensureBankQuestion(ctx, pool, questionbankMod, versionID, item)
			if err != nil {
				return fmt.Errorf("bank %s: %w", item.Slug, err)
			}
			if created {
				questions++
			}
		}

		composed, err := composeFixedTests(ctx, pool, examMod, file.ExamVersion)
		if err != nil {
			return fmt.Errorf("compose fixed tests for %s: %w", file.ExamVersion, err)
		}
		_, _ = fmt.Fprintf(out, "  ✓ %s: %d published, %d drafted, %d question(s) added, %d fixed test(s) composed\n",
			file.ExamVersion, published, drafted, questions, composed)
	}
	return nil
}

// authorExamItem authors and, when the fixture says it was confirmed, publishes
// the item's content version. It reports whether the item is published.
func authorExamItem(
	ctx context.Context,
	pool *pgxpool.Pool,
	author contentcontract.Author,
	recorder contentcontract.VerificationRecorder,
	adminID uuid.UUID,
	item examfixture.ExamItem,
) (uuid.UUID, bool, error) {
	body, found, err := publishedBodyBySlug(ctx, pool, item.Slug)
	if err != nil {
		return uuid.Nil, false, err
	}
	if found && jsonBodiesEqual(body, item.Body) {
		// Already published and unchanged: reuse its current version.
		versionID, err := publishedVersionIDBySlug(ctx, pool, item.Slug)
		return versionID, err == nil, err
	}

	spec := contentcontract.AuthorSpec{
		Slug:      item.Slug,
		Kind:      item.Kind,
		CEFRLevel: item.CEFRLevel,
		Body:      item.Body,
		AuthorID:  adminID,
	}
	versionID, err := author.EnsureDraft(ctx, spec)
	if err != nil {
		return uuid.Nil, false, err
	}

	verification := contentcontract.Verification{
		Confirmed: item.Verification.Confirmed,
		Model:     item.Verification.Model,
		Reason:    item.Verification.Reason,
		CheckedAt: item.Verification.CheckedAt,
	}
	if !verification.Confirmed {
		if err := recorder.RecordVerification(ctx, versionID, verification); err != nil {
			return uuid.Nil, false, err
		}
		return versionID, false, nil
	}
	if err := author.ApproveVerified(ctx, versionID, verification); err != nil {
		return uuid.Nil, false, err
	}
	return versionID, true, nil
}

// ensureBankQuestion creates the bank row for a published item, if it is not
// already there, and puts it in the bank. It reports whether a row was created.
func ensureBankQuestion(
	ctx context.Context,
	pool *pgxpool.Pool,
	questionbankMod *questionbank.Module,
	versionID uuid.UUID,
	item examfixture.ExamItem,
) (bool, error) {
	fingerprint, err := questionbankdomain.FingerprintFromBody(item.Kind, item.Body)
	if err != nil {
		return false, fmt.Errorf("fingerprint: %w", err)
	}

	var existing uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id FROM assess.questions WHERE fingerprint = $1", fingerprint).Scan(&existing)
	if err == nil {
		return false, nil
	}
	if err != pgx.ErrNoRows {
		return false, err
	}

	itemID, err := contentItemIDByVersion(ctx, pool, versionID)
	if err != nil {
		return false, err
	}
	partID, err := uuid.Parse(item.ExamPartID)
	if err != nil {
		return false, fmt.Errorf("exam_part_id %q: %w", item.ExamPartID, err)
	}

	created, err := questionbankMod.Author().CreateQuestion(ctx, &questionbankcontract.Question{
		ContentItemID: itemID,
		ExamPartID:    &partID,
		Kind:          item.Kind,
		Skill:         item.Skill,
		CEFRLevel:     item.CEFRLevel,
		QuestionCount: 1,
		Fingerprint:   fingerprint,
		Provenance:    map[string]any{"content_version_id": versionID.String()},
		Status:        questionbankdomain.StatusDraft,
	})
	if err != nil {
		return false, err
	}
	if _, err := questionbankMod.Author().PublishQuestion(ctx, created.ID); err != nil {
		return false, err
	}
	return true, nil
}

// composeFixedTests composes the next numbered tests each blueprint of the exam
// version can fill, and returns how many it composed.
func composeFixedTests(
	ctx context.Context, pool *pgxpool.Pool, examMod *exam.Module, versionCode string,
) (int, error) {
	rows, err := pool.Query(ctx, `
		SELECT b.id FROM assess.blueprints b
		JOIN assess.exam_versions ev ON ev.id = b.version_id
		WHERE ev.code = $1`, versionCode)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var blueprintIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		blueprintIDs = append(blueprintIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	composed := 0
	for _, blueprintID := range blueprintIDs {
		count, err := examMod.Service().ComposeNextFixedTests(ctx, blueprintID)
		if err != nil {
			return composed, err
		}
		composed += count
	}
	return composed, nil
}

// publishedVersionIDBySlug reads the current published version id of an item.
func publishedVersionIDBySlug(ctx context.Context, pool *pgxpool.Pool, slug string) (uuid.UUID, error) {
	const query = `
		SELECT i.current_version_id
		FROM content.content_items i
		WHERE i.slug = $1 AND i.status = 'published'`
	var id uuid.UUID
	if err := pool.QueryRow(ctx, query, slug).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// contentItemIDByVersion reads the content item a version belongs to.
func contentItemIDByVersion(ctx context.Context, pool *pgxpool.Pool, versionID uuid.UUID) (uuid.UUID, error) {
	const query = "SELECT item_id FROM content.content_versions WHERE id = $1"
	var id uuid.UUID
	if err := pool.QueryRow(ctx, query, versionID).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// offlineRBAC is the permissive authorizer an offline seed gives the
// questionbank: nothing here serves a request, and the command mounts no router
// to protect.
type offlineRBAC struct{}

func (offlineRBAC) Require(_ context.Context, _ rbaccontract.Permission) error { return nil }
func (offlineRBAC) Can(_ context.Context, _ rbaccontract.Permission) bool      { return true }

// offlineGuard is the permissive lesson guard an offline seed gives the lesson
// module, which has no authoring-only constructor.
type offlineGuard struct{}

func (offlineGuard) Require(_ context.Context, _ string) error { return nil }
