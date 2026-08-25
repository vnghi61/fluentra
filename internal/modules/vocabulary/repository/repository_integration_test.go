//go:build integration

// Package repository_test exercises every vocabulary query against a real
// PostgreSQL instance.
//
// The service tests drive a fake repository, which proves the orchestration and
// nothing about the SQL. The queries here carry the parts a fake cannot model at
// all: two ON CONFLICT clauses with opposite intent, a LEFT JOIN aggregate, a
// visibility predicate that mixes a learner's own decks with curated ones, and a
// prefix search whose count has to agree with its page.
package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

const testDatabase = "fluentra_vocabulary_repository_test"

// statusKnown is the status that also stops srs scheduling, which is why it is
// the one the state tests move to.
const statusKnown = "known"

var packagePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, testDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", testDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", testDatabase, err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", testDatabase, err)
		os.Exit(1)
	}
	packagePool = pool

	code := m.Run()

	pool.Close()
	dropDatabase()
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
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, drop); err != nil {
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
		_, _ = cleanup.ExecContext(context.Background(), drop)
	}, nil
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

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

func newRepo(ctx context.Context, t *testing.T) (repository.Repository, uuid.UUID) {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	var userID uuid.UUID
	email := fmt.Sprintf("vocab-repo-%s@example.com", uuid.NewString())
	const insert = `INSERT INTO core.users (email, status) VALUES ($1, 'active') RETURNING id`
	if err := packagePool.QueryRow(ctx, insert, email).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = packagePool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID)
	})

	return repository.New(packagePool), userID
}

// uniqueLemma keeps tests from colliding on uq_words_lemma_pos when they run in
// the same database.
func uniqueLemma(prefix string) string {
	return prefix + uuid.NewString()[:8]
}

func seedWord(ctx context.Context, t *testing.T, repo repository.Repository, lemma, pos string) sqlc.SkillWord {
	t.Helper()

	word, err := repo.InsertWord(ctx, sqlc.InsertWordParams{
		Lemma: lemma, Pos: pos, CefrLevel: "B2",
	})
	if err != nil {
		t.Fatalf("seed word %s: %v", lemma, err)
	}
	t.Cleanup(func() {
		_, _ = packagePool.Exec(ctx, `DELETE FROM skill.words WHERE id = $1`, word.ID)
	})
	return word
}

func seedSense(
	ctx context.Context, t *testing.T, repo repository.Repository, wordID uuid.UUID, definition string,
) sqlc.SkillWordSense {
	t.Helper()

	sense, err := repo.InsertWordSense(ctx, sqlc.InsertWordSenseParams{
		WordID:     wordID,
		Definition: definition,
		Examples:   []byte(`[{"sentence":"She kept meticulous records."}]`),
	})
	if err != nil {
		t.Fatalf("seed sense: %v", err)
	}
	return sense
}

// TestRepository_InsertWordUpdatesOnConflict: unlike a review card, a dictionary
// entry is authored data and re-importing it must refresh it. The two ON CONFLICT
// clauses in this module have opposite intent on purpose, and this pins one half.
func TestRepository_InsertWordUpdatesOnConflict(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(ctx, t)

	lemma := uniqueLemma("meticulous-")
	first := seedWord(ctx, t, repo, lemma, "adjective")

	rank := int32(4821)
	ipa := "/məˈtɪkjələs/"
	second, err := repo.InsertWord(ctx, sqlc.InsertWordParams{
		Lemma: lemma, Pos: "adjective", CefrLevel: "C1", FrequencyRank: &rank, Ipa: &ipa,
	})
	if err != nil {
		t.Fatalf("re-import word: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("re-importing created a duplicate word %s", second.ID)
	}
	if second.CefrLevel != "C1" {
		t.Errorf("cefr_level = %s, want C1 — a re-import must refresh the entry", second.CefrLevel)
	}
	if second.Ipa == nil || *second.Ipa != ipa {
		t.Errorf("ipa did not update on re-import: %v", second.Ipa)
	}
}

// TestRepository_WordLookupAndSearch covers GetWordByLemmaAndPOS, GetWordByID,
// ListWordsByLemma, SearchWords and CountSearchWords — including the rule that
// the count must agree with the page it describes.
func TestRepository_WordLookupAndSearch(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(ctx, t)

	// One lemma, two parts of speech: this is why lookup returns a list.
	lemma := uniqueLemma("bank-")
	noun := seedWord(ctx, t, repo, lemma, "noun")
	seedWord(ctx, t, repo, lemma, "verb")

	byLemmaAndPOS, err := repo.GetWordByLemmaAndPOS(ctx, lemma, "noun")
	if err != nil {
		t.Fatalf("get by lemma and pos: %v", err)
	}
	if byLemmaAndPOS.ID != noun.ID {
		t.Errorf("got word %s, want %s", byLemmaAndPOS.ID, noun.ID)
	}

	byID, err := repo.GetWordByID(ctx, noun.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Lemma != lemma {
		t.Errorf("got lemma %s, want %s", byID.Lemma, lemma)
	}

	all, err := repo.ListWordsByLemma(ctx, lemma)
	if err != nil {
		t.Fatalf("list by lemma: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("listed %d entries for %q, want 2 — a lemma is not unique on its own", len(all), lemma)
	}

	page, err := repo.SearchWords(ctx, lemma, 1, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page) != 1 {
		t.Errorf("a page of 1 returned %d rows", len(page))
	}

	total, err := repo.CountSearchWords(ctx, lemma)
	if err != nil {
		t.Fatalf("count search: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2 — the count must describe the whole result, not the page", total)
	}

	if _, err := repo.GetWordByID(ctx, uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("looking up a word that does not exist returned %v, want pgx.ErrNoRows", err)
	}
}

// TestRepository_SensesAndRelations covers InsertWordSense, ListSensesByWordID,
// GetSenseByID, ListSensesByIDs, InsertWordRelation and ListRelationsByWordID.
func TestRepository_SensesAndRelations(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(ctx, t)

	word := seedWord(ctx, t, repo, uniqueLemma("meticulous-"), "adjective")
	first := seedSense(ctx, t, repo, word.ID, "Showing great attention to detail.")
	second := seedSense(ctx, t, repo, word.ID, "Very careful and precise.")

	senses, err := repo.ListSensesByWordID(ctx, word.ID)
	if err != nil {
		t.Fatalf("list senses: %v", err)
	}
	if len(senses) != 2 {
		t.Errorf("listed %d senses, want 2", len(senses))
	}

	// GetSenseByID joins the word, because a sense on its own cannot render a
	// flashcard: it has no lemma, no part of speech and no IPA.
	detail, err := repo.GetSenseByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("get sense: %v", err)
	}
	if detail.Lemma != word.Lemma || detail.Pos != word.Pos {
		t.Errorf("GetSenseByID lost the word: lemma %q pos %q", detail.Lemma, detail.Pos)
	}
	if len(detail.Examples) == 0 {
		t.Error("the sense lost its examples")
	}

	batched, err := repo.ListSensesByIDs(ctx, []uuid.UUID{first.ID, second.ID})
	if err != nil {
		t.Fatalf("list senses by ids: %v", err)
	}
	if len(batched) != 2 {
		t.Errorf("batched read returned %d senses, want 2", len(batched))
	}

	other := seedWord(ctx, t, repo, uniqueLemma("careless-"), "adjective")
	if _, err := repo.InsertWordRelation(ctx, sqlc.InsertWordRelationParams{
		FromWordID: word.ID, ToWordID: other.ID, Relation: "antonym",
	}); err != nil {
		t.Fatalf("insert relation: %v", err)
	}

	relations, err := repo.ListRelationsByWordID(ctx, word.ID)
	if err != nil {
		t.Fatalf("list relations: %v", err)
	}
	if len(relations) != 1 {
		t.Fatalf("listed %d relations, want 1", len(relations))
	}
	if relations[0].Relation != "antonym" || relations[0].TargetLemma != other.Lemma {
		t.Errorf("relation lost its target: %+v", relations[0])
	}
}

// TestRepository_DeckVisibility is the rule the deck list depends on: a learner
// sees their own decks and the curated public ones, and nobody else's.
func TestRepository_DeckVisibility(t *testing.T) {
	ctx := context.Background()
	repo, owner := newRepo(ctx, t)
	_, stranger := newRepo(ctx, t)

	mine, err := repo.InsertDeck(ctx, sqlc.InsertDeckParams{
		OwnerID: &owner, Slug: uniqueLemma("mine-"), Name: "My Deck",
	})
	if err != nil {
		t.Fatalf("insert my deck: %v", err)
	}
	theirs, err := repo.InsertDeck(ctx, sqlc.InsertDeckParams{
		OwnerID: &stranger, Slug: uniqueLemma("theirs-"), Name: "Their Deck",
	})
	if err != nil {
		t.Fatalf("insert their deck: %v", err)
	}
	// A curated deck has no owner and is public.
	curated, err := repo.InsertDeck(ctx, sqlc.InsertDeckParams{
		OwnerID: nil, Slug: uniqueLemma("curated-"), Name: "Core A1", IsPublic: true,
	})
	if err != nil {
		t.Fatalf("insert curated deck: %v", err)
	}

	visible, err := repo.ListDecksByUser(ctx, &owner)
	if err != nil {
		t.Fatalf("list decks: %v", err)
	}

	seen := make(map[uuid.UUID]bool, len(visible))
	for _, deck := range visible {
		seen[deck.ID] = true
	}
	if !seen[mine.ID] {
		t.Error("a learner cannot see their own deck")
	}
	if !seen[curated.ID] {
		t.Error("a learner cannot see the curated decks")
	}
	if seen[theirs.ID] {
		t.Error("a learner can see another learner's deck")
	}

	byID, err := repo.GetDeckByID(ctx, mine.ID)
	if err != nil {
		t.Fatalf("get deck: %v", err)
	}
	if byID.Name != "My Deck" {
		t.Errorf("got deck %q", byID.Name)
	}
}

// TestRepository_DeckMembership covers InsertDeckItem, ListDeckWords and
// DeleteDeckItem, plus the ON CONFLICT DO NOTHING that makes adding a word twice
// a no-op the service reports as a conflict rather than a duplicate row.
func TestRepository_DeckMembership(t *testing.T) {
	ctx := context.Background()
	repo, owner := newRepo(ctx, t)

	word := seedWord(ctx, t, repo, uniqueLemma("meticulous-"), "adjective")
	sense := seedSense(ctx, t, repo, word.ID, "Showing great attention to detail.")

	deck, err := repo.InsertDeck(ctx, sqlc.InsertDeckParams{
		OwnerID: &owner, Slug: uniqueLemma("deck-"), Name: "Deck",
	})
	if err != nil {
		t.Fatalf("insert deck: %v", err)
	}

	if _, err := repo.InsertDeckItem(ctx, sqlc.InsertDeckItemParams{
		DeckID: deck.ID, WordSenseID: sense.ID,
	}); err != nil {
		t.Fatalf("add word to deck: %v", err)
	}

	// DO NOTHING returns no row, which is how the service knows to answer
	// WORD_ALREADY_IN_DECK instead of pretending it added it again.
	_, err = repo.InsertDeckItem(ctx, sqlc.InsertDeckItemParams{DeckID: deck.ID, WordSenseID: sense.ID})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("adding the same sense twice returned %v, want pgx.ErrNoRows", err)
	}

	items, err := repo.ListDeckWords(ctx, deck.ID, 20, 0)
	if err != nil {
		t.Fatalf("list deck words: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("listed %d deck items, want 1", len(items))
	}
	// P10.4 renders these fields; the join has to carry all of them.
	if items[0].Lemma != word.Lemma || items[0].Definition == "" || len(items[0].Examples) == 0 {
		t.Errorf("a deck item lost the fields a flashcard renders: %+v", items[0])
	}

	if err := repo.DeleteDeckItem(ctx, deck.ID, sense.ID); err != nil {
		t.Fatalf("remove word from deck: %v", err)
	}
	items, err = repo.ListDeckWords(ctx, deck.ID, 20, 0)
	if err != nil {
		t.Fatalf("list after removal: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("%d items survived removal", len(items))
	}
}

// TestRepository_UserWordState covers UpsertUserWordState and GetUserWordState,
// including that the upsert moves the status rather than inserting a second row.
func TestRepository_UserWordState(t *testing.T) {
	ctx := context.Background()
	repo, userID := newRepo(ctx, t)

	word := seedWord(ctx, t, repo, uniqueLemma("ephemeral-"), "adjective")
	sense := seedSense(ctx, t, repo, word.ID, "Lasting for a very short time.")

	learning, err := repo.UpsertUserWordState(ctx, sqlc.UpsertUserWordStateParams{
		UserID: userID, WordSenseID: sense.ID, Status: "learning",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	known, err := repo.UpsertUserWordState(ctx, sqlc.UpsertUserWordStateParams{
		UserID: userID, WordSenseID: sense.ID, Status: statusKnown,
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if known.ID != learning.ID {
		t.Error("marking a word known created a second state row")
	}
	if known.Status != statusKnown {
		t.Errorf("status = %s, want known", known.Status)
	}
	if !known.FirstSeenAt.Equal(learning.FirstSeenAt) {
		t.Error("first_seen_at moved; it records when the learner first met the sense, not the last update")
	}

	read, err := repo.GetUserWordState(ctx, userID, sense.ID)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if read.Status != statusKnown {
		t.Errorf("read back status %s, want known", read.Status)
	}

	if _, err := repo.GetUserWordState(ctx, userID, uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a sense the learner never met returned %v, want pgx.ErrNoRows", err)
	}
}
