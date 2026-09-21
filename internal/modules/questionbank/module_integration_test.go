//go:build integration

package questionbank_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/db/migrations"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/domain"
	"github.com/fluentra/fluentra/internal/modules/questionbank/repository"
	"github.com/fluentra/fluentra/internal/modules/questionbank/service"
)

const (
	moduleDatabase = "fluentra_questionbank_module_test"
	kindReading    = "reading_comprehension"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, drop, err := createDatabase(base, moduleDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		drop()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		drop()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	pool = created

	code := m.Run()

	pool.Close()
	drop()
	os.Exit(code)
}

func createDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceDatabase(base, "postgres")
	if err != nil {
		return "", nil, err
	}
	admin, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = admin.Close() }()

	ctx := context.Background()
	dropStmt := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, dropStmt); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}
	dsn, err := replaceDatabase(base, name)
	if err != nil {
		return "", nil, err
	}
	return dsn, func() {
		cleanup, err := sql.Open("pgx", maintenance)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), dropStmt)
	}, nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

func migrateUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("create goose provider: %w", err)
	}
	defer func() { _ = provider.Close() }()
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func requirePool(t *testing.T) {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
}

type recordingLessonAuthor struct {
	mockLessonAuthorBase
	appended []lessoncontract.ActivitySpec
}

type mockLessonAuthorBase struct{}

func (mockLessonAuthorBase) EnsureCourse(context.Context, lessoncontract.CourseSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (mockLessonAuthorBase) EnsureUnit(context.Context, lessoncontract.UnitSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (mockLessonAuthorBase) EnsureLesson(context.Context, lessoncontract.LessonSpec) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (mockLessonAuthorBase) SyncActivities(context.Context, uuid.UUID, []lessoncontract.ActivitySpec) error {
	return nil
}

func (mockLessonAuthorBase) UpdateLessonStatus(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *recordingLessonAuthor) AppendActivity(
	_ context.Context, _ uuid.UUID, spec lessoncontract.ActivitySpec,
) (uuid.UUID, error) {
	r.appended = append(r.appended, spec)
	return uuid.New(), nil
}

// seedQuestion stores a generated draft question whose content version has the
// given status, the way GenerateQuestions leaves it.
func seedQuestion(
	t *testing.T, repo *repository.Repository, content *memoryContentReader, partID uuid.UUID, versionStatus string,
) (*domain.Question, *contentcontract.Version) {
	t.Helper()
	version := &contentcontract.Version{
		ID:     uuid.New(),
		ItemID: uuid.New(),
		Kind:   kindReading,
		Body:   json.RawMessage(`{"passage":"p","questions":[{"id":"q1"}]}`),
		Status: versionStatus,
	}
	content.versions[version.ID] = version

	q, err := repo.CreateQuestion(context.Background(), &domain.Question{
		ID:            uuid.New(),
		ContentItemID: version.ItemID,
		ExamPartID:    &partID,
		Kind:          kindReading,
		Skill:         "reading",
		CEFRLevel:     "B2",
		QuestionCount: 1,
		Fingerprint:   uuid.NewString(),
		Provenance: map[string]any{
			"prompt_version":     "item_generate.v1",
			"model":              "test",
			"content_version_id": version.ID.String(),
		},
		Status:    domain.StatusDraft,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	return q, version
}

// A bank question enters the bank only after a person approves its content
// (brief §8): PublishQuestion on an unreviewed draft is refused, and approving
// the content — the content.published event — is what makes it drawable.
func TestBank_AQuestionIsDrawableOnlyAfterItsContentIsReviewed(t *testing.T) {
	requirePool(t)
	ctx := context.Background()

	partID := uuid.MustParse("20000000-0000-0000-0002-000000000004") // VSTEP reading part 1
	repo := repository.New(pool)
	content := &memoryContentReader{versions: map[uuid.UUID]*contentcontract.Version{}}
	lessons := &recordingLessonAuthor{}
	svc := service.New(service.Config{Repo: repo, ContentReader: content, LessonAuthor: lessons})

	q, version := seedQuestion(t, repo, content, partID, "draft")

	_, err := svc.PublishQuestion(ctx, q.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNotReviewed), "got %v", err)
	assert.Empty(t, lessons.appended, "nothing reaches the bank course before review")

	drawable, err := svc.DrawableForPart(ctx, partID)
	require.NoError(t, err)
	assert.Empty(t, drawable)

	// A reviewer approves it in the review queue; content publishes it.
	version.Status = "published"
	require.NoError(t, svc.HandleContentPublished(ctx, contentcontract.Published{
		ItemID: version.ItemID, VersionID: version.ID,
	}))

	require.Len(t, lessons.appended, 1)
	assert.Equal(t, version.ID, lessons.appended[0].ContentVersionID)
	assert.JSONEq(t, string(version.Body), string(lessons.appended[0].Config))

	drawable, err = svc.DrawableForPart(ctx, partID)
	require.NoError(t, err)
	require.Len(t, drawable, 1)
	assert.Equal(t, q.ID, drawable[0].ID)
	assert.NotNil(t, drawable[0].ActivityID)

	// Redelivery of the event does not append the activity twice.
	require.NoError(t, svc.HandleContentPublished(ctx, contentcontract.Published{
		ItemID: version.ItemID, VersionID: version.ID,
	}))
	assert.Len(t, lessons.appended, 1)
}

func TestBank_ContentThatIsNotABankQuestionIsIgnored(t *testing.T) {
	requirePool(t)
	svc := service.New(service.Config{
		Repo:          repository.New(pool),
		ContentReader: &memoryContentReader{versions: map[uuid.UUID]*contentcontract.Version{}},
		LessonAuthor:  &recordingLessonAuthor{},
	})

	err := svc.HandleContentPublished(context.Background(), contentcontract.Published{
		ItemID: uuid.New(), VersionID: uuid.New(),
	})

	assert.NoError(t, err)
}

// DB4: assess references only its own schema and core.users. 1700000850 broke
// this and nothing checked it.
func TestAssessSchema_CrossSchemaForeignKeysRestrictedToUsers(t *testing.T) {
	requirePool(t)

	rows, err := pool.Query(context.Background(), `
		SELECT t.relname, c.conname, fn.nspname, ft.relname
		FROM pg_constraint c
		JOIN pg_class t      ON t.oid  = c.conrelid
		JOIN pg_namespace n  ON n.oid  = t.relnamespace
		JOIN pg_class ft     ON ft.oid = c.confrelid
		JOIN pg_namespace fn ON fn.oid = ft.relnamespace
		WHERE c.contype = 'f' AND n.nspname = 'assess'`)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var table, constraint, schema, target string
		require.NoError(t, rows.Scan(&table, &constraint, &schema, &target))
		if schema == "assess" || (schema == "core" && target == "users") {
			continue
		}
		t.Errorf("DB4 violation: assess.%s constraint %s references %s.%s", table, constraint, schema, target)
	}
	require.NoError(t, rows.Err())
}
