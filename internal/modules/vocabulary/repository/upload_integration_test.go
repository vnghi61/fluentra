//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// The upload queries behind search, paging, the per-term lock and the reuse of
// another learner's verdict. The service tests drive a fake, which cannot see
// a keyset off by one, a LIKE wildcard or a lock that outlives its transaction.

// seedUploadItems stores one upload of the given terms for the user, in one
// transaction as Submit does — which is what gives every word of an upload the
// same created_at, and the search its alphabetical order within one.
func seedUploadItems(
	ctx context.Context, t *testing.T, base repository.Repository, userID uuid.UUID, terms ...string,
) (sqlc.SkillVocabUpload, []sqlc.SkillVocabUploadItem) {
	t.Helper()
	tx, err := packagePool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo := base.WithTx(tx)
	upload, err := repo.InsertUpload(ctx, sqlc.InsertUploadParams{
		UserID: userID, RawText: "seed", ItemCount: int32(len(terms)), //nolint:gosec // a handful of terms
	})
	if err != nil {
		t.Fatalf("insert upload: %v", err)
	}
	items := make([]sqlc.SkillVocabUploadItem, 0, len(terms))
	for _, term := range terms {
		item, err := repo.InsertUploadItem(ctx, sqlc.InsertUploadItemParams{
			UploadID: upload.ID, UserID: userID, Term: term,
		})
		if err != nil {
			t.Fatalf("insert item %s: %v", term, err)
		}
		items = append(items, item)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return upload, items
}

// TestRepository_SearchUserUploadItems pins the filters and the keyset: a page
// boundary inside one upload (one created_at for every word) is where an off by
// one would repeat or drop a word.
func TestRepository_SearchUserUploadItems(t *testing.T) {
	ctx := context.Background()
	repo, userID := newRepo(ctx, t)
	first, _ := seedUploadItems(ctx, t, repo, userID, "apple", "banana", "cherry", "100%_sure")
	_, second := seedUploadItems(ctx, t, repo, userID, "bandana")
	note := "not_a_word"
	if _, err := repo.MarkUploadItemRejected(ctx, second[0].ID, "Not a word.", nil, &note); err != nil {
		t.Fatalf("reject: %v", err)
	}

	search := func(params sqlc.SearchUserUploadItemsParams) []sqlc.SearchUserUploadItemsRow {
		t.Helper()
		params.UserID = userID
		if params.Statuses == nil {
			params.Statuses = []string{}
		}
		if params.RowLimit == 0 {
			params.RowLimit = 100
		}
		rows, err := repo.SearchUserUploadItems(ctx, params)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		return rows
	}
	terms := func(rows []sqlc.SearchUserUploadItemsRow) string {
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			out = append(out, row.Term)
		}
		return strings.Join(out, ",")
	}

	if got := terms(search(sqlc.SearchUserUploadItemsParams{Query: "AN"})); got != "bandana,banana" {
		t.Errorf("case-insensitive substring, newest upload first: got %s", got)
	}
	if got := terms(search(sqlc.SearchUserUploadItemsParams{Statuses: []string{"rejected"}})); got != "bandana" {
		t.Errorf("status filter: got %s", got)
	}
	byUpload := search(sqlc.SearchUserUploadItemsParams{UploadID: &first.ID})
	if got := terms(byUpload); got != "100%_sure,apple,banana,cherry" {
		t.Errorf("one upload, alphabetical: got %s", got)
	}
	// The service escapes LIKE wildcards before they arrive here.
	if got := terms(search(sqlc.SearchUserUploadItemsParams{Query: `\%`})); got != "100%_sure" {
		t.Errorf("escaped percent: got %s", got)
	}

	page := search(sqlc.SearchUserUploadItemsParams{UploadID: &first.ID, RowLimit: 2})
	if len(page) != 2 {
		t.Fatalf("first page has %d rows, want 2", len(page))
	}
	last := page[1]
	rest := search(sqlc.SearchUserUploadItemsParams{
		UploadID: &first.ID, AfterCreatedAt: &last.CreatedAt, AfterTerm: &last.Term, AfterID: &last.ID,
	})
	if got := terms(append(page, rest...)); got != "100%_sure,apple,banana,cherry" {
		t.Errorf("pages joined = %s, want every word once, in order", got)
	}
}

// TestRepository_ListWordsWithSensesByLemmas is the database-first check every
// upload starts with. It named a deck_items column that does not exist
// (sense_id for word_sense_id) inside an EXISTS, which sqlc does not check, so
// it failed to prepare on every call and every known word a learner pasted
// ended up "failed". Only running it against Postgres shows that.
func TestRepository_ListWordsWithSensesByLemmas(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(ctx, t)
	lemma := uniqueLemma("known-")
	word := seedWord(ctx, t, repo, lemma, "noun")
	inDeck := seedSense(ctx, t, repo, word.ID, "The sense a curated deck carries.")
	seedSense(ctx, t, repo, word.ID, "A sense no deck carries.")

	deck, err := repo.InsertDeck(ctx, sqlc.InsertDeckParams{
		Slug: uniqueLemma("curated-"), Name: "Curated", IsPublic: true,
	})
	if err != nil {
		t.Fatalf("insert deck: %v", err)
	}
	t.Cleanup(func() { _, _ = packagePool.Exec(ctx, `DELETE FROM skill.decks WHERE id = $1`, deck.ID) })
	if _, err := repo.InsertDeckItem(ctx, sqlc.InsertDeckItemParams{DeckID: deck.ID, WordSenseID: inDeck.ID}); err != nil {
		t.Fatalf("insert deck item: %v", err)
	}

	rows, err := repo.ListWordsWithSensesByLemmas(ctx, []string{lemma, uniqueLemma("absent-")})
	if err != nil {
		t.Fatalf("list by lemmas: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want both senses of %s and nothing for the absent lemma", len(rows), lemma)
	}
	for _, row := range rows {
		if want := row.SenseID == inDeck.ID; row.InPublicDeck != want {
			t.Errorf("sense %s in_public_deck = %v, want %v", row.SenseID, row.InPublicDeck, want)
		}
	}
}

// TestRepository_FindRecentTermRejection: a verdict is found by normalised term.
func TestRepository_FindRecentTermRejection(t *testing.T) {
	ctx := context.Background()
	repo, userID := newRepo(ctx, t)
	term := uniqueLemma("zxq")
	_, items := seedUploadItems(ctx, t, repo, userID, "  "+strings.ToUpper(term)+" ")
	note := "not_a_word"
	if _, err := repo.MarkUploadItemRejected(ctx, items[0].ID, "Not a word.", nil, &note); err != nil {
		t.Fatalf("reject: %v", err)
	}

	prior, err := repo.FindRecentTermRejection(ctx, term)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if prior.NoteCode == nil || *prior.NoteCode != note {
		t.Errorf("note = %v, want %s", prior.NoteCode, note)
	}
	if _, err := repo.FindRecentTermRejection(ctx, uniqueLemma("never-")); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("an unseen term returned %v, want pgx.ErrNoRows", err)
	}
}

// TestRepository_ClaimByItemIDs: a job given words claims only those.
func TestRepository_ClaimByItemIDs(t *testing.T) {
	ctx := context.Background()
	repo, userID := newRepo(ctx, t)
	upload, items := seedUploadItems(ctx, t, repo, userID, "one", "two", "three")

	claimed, err := repo.ClaimPendingUploadItemsByUploadID(ctx, upload.ID, []uuid.UUID{items[1].ID}, 3, 50)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != items[1].ID {
		t.Errorf("claimed %d words, want only %s", len(claimed), items[1].ID)
	}
	all, err := repo.ClaimPendingUploadItemsByUploadID(ctx, upload.ID, nil, 3, 50)
	if err != nil || len(all) != 3 {
		t.Errorf("no filter claimed %d (%v), want every pending word", len(all), err)
	}
}

// TestRepository_ListUploadsByUserPagesAndCountsFailures.
func TestRepository_ListUploadsByUserPagesAndCountsFailures(t *testing.T) {
	ctx := context.Background()
	repo, userID := newRepo(ctx, t)
	older, items := seedUploadItems(ctx, t, repo, userID, "alpha")
	if err := repo.RecordUploadItemAttempt(ctx, items[0].ID, 1, "gave up"); err != nil {
		t.Fatalf("fail item: %v", err)
	}
	newer, _ := seedUploadItems(ctx, t, repo, userID, "beta")

	first, err := repo.ListUploadsByUser(ctx, sqlc.ListUploadsByUserParams{UserID: userID, RowLimit: 1})
	if err != nil || len(first) != 1 || first[0].ID != newer.ID {
		t.Fatalf("first page = %d rows (%v), want the newer upload", len(first), err)
	}
	second, err := repo.ListUploadsByUser(ctx, sqlc.ListUploadsByUserParams{
		UserID: userID, BeforeCreatedAt: &first[0].CreatedAt, BeforeID: &first[0].ID, RowLimit: 10,
	})
	if err != nil || len(second) != 1 || second[0].ID != older.ID {
		t.Fatalf("second page = %d rows (%v), want the older upload", len(second), err)
	}
	if second[0].FailedCount != 1 {
		t.Errorf("failed_count = %d, want 1", second[0].FailedCount)
	}
}

// TestRepository_LockUploadTermHoldsUntilTheTransactionEnds.
func TestRepository_LockUploadTermHoldsUntilTheTransactionEnds(t *testing.T) {
	ctx := context.Background()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	repo := repository.New(packagePool)
	term := uniqueLemma("lock-")
	key := fmt.Sprintf("vocab-upload-term:%s", term)
	const try = `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`
	const release = `SELECT pg_advisory_unlock(hashtextextended($1, 0))`

	tx, err := packagePool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := repo.WithTx(tx).LockUploadTerm(ctx, term); err != nil {
		t.Fatalf("lock: %v", err)
	}

	other, err := packagePool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer other.Release()
	var free bool
	if err := other.QueryRow(ctx, try, key).Scan(&free); err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if free {
		t.Error("another session took the term while the first one held it")
		_, _ = other.Exec(ctx, release, key)
	}

	_ = tx.Rollback(ctx)
	if err := other.QueryRow(ctx, try, key).Scan(&free); err != nil {
		t.Fatalf("try lock after rollback: %v", err)
	}
	if !free {
		t.Error("the lock outlived its transaction")
	}
	_, _ = other.Exec(ctx, release, key)
}
