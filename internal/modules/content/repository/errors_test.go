package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// errRow hands every Scan the injected error, which is how sqlc's `:one`
// methods surface a failed query.
type errRow struct{ err error }

func (r errRow) Scan(_ ...any) error { return r.err }

// errQuerier fails every statement with the same error, so a test can drive one
// repository method into its error branch without a database.
type errQuerier struct{ err error }

func (q errQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), q.err
}

func (q errQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, q.err
}

func (q errQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return errRow(q)
}

func repoFailingWith(err error) *repository.Repository {
	return repository.New(errQuerier{err: err})
}

// TestNoRowsBecomesADomainError is the "documented apperr code, not a 500"
// acceptance at the repository boundary: pgx.ErrNoRows must never escape as a
// bare error, because the HTTP layer turns anything unrecognised into a 500.
func TestNoRowsBecomesADomainError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(context.Context, *repository.Repository) error
		want error
	}{
		{
			name: "GetItemByID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetItemByID(ctx, uuid.New())
				return err
			},
			want: domain.ErrItemNotFound,
		},
		{
			name: "GetItemBySlug",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetItemBySlug(ctx, "some-slug")
				return err
			},
			want: domain.ErrItemNotFound,
		},
		{
			name: "UpdateItemStatus",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.UpdateItemStatus(ctx, uuid.New(), domain.StatusPublished)
				return err
			},
			want: domain.ErrItemNotFound,
		},
		{
			name: "UpdateItemCurrentVersion",
			call: func(ctx context.Context, r *repository.Repository) error {
				id := uuid.New()
				_, err := r.UpdateItemCurrentVersion(ctx, uuid.New(), &id)
				return err
			},
			want: domain.ErrItemNotFound,
		},
		{
			name: "GetVersionByID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetVersionByID(ctx, uuid.New())
				return err
			},
			want: domain.ErrVersionNotFound,
		},
		{
			name: "GetVersionByItemAndVersion",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetVersionByItemAndVersion(ctx, uuid.New(), 1)
				return err
			},
			want: domain.ErrVersionNotFound,
		},
		{
			name: "GetDraftVersionByItemID",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetDraftVersionByItemID(ctx, uuid.New())
				return err
			},
			want: domain.ErrVersionNotFound,
		},
		{
			name: "PublishVersion",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.PublishVersion(ctx, uuid.New())
				return err
			},
			want: domain.ErrVersionNotFound,
		},
		{
			name: "UpdateVersionDraft",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.UpdateVersionDraft(
					ctx, uuid.New(), "vocab_word", []byte("{}"), domain.CEFRB1, nil, domain.StatusDraft,
				)
				return err
			},
			want: domain.ErrVersionNotFound,
		},
		{
			name: "GetPublishedVersionBySlug",
			call: func(ctx context.Context, r *repository.Repository) error {
				_, err := r.GetPublishedVersionBySlug(ctx, "some-slug")
				return err
			},
			want: domain.ErrContentNotPublished,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := testCase.call(context.Background(), repoFailingWith(pgx.ErrNoRows))
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestCreateItemMapsTheSlugUniqueViolation keeps a duplicate slug a 409 rather
// than a 500 carrying a raw constraint name.
func TestCreateItemMapsTheSlugUniqueViolation(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "uq_content_items_slug"}
	repo := repoFailingWith(pgErr)

	_, err := repo.CreateItem(
		context.Background(), uuid.New(), "vocab_word", "taken-slug", domain.StatusDraft, uuid.New(),
	)
	if !errors.Is(err, domain.ErrSlugAlreadyExists) {
		t.Fatalf("error = %v, want ErrSlugAlreadyExists", err)
	}
}

// TestUpdateVersionDraftMapsTheImmutabilityTrigger covers the other half of the
// P7.2 guarantee: trg_content_versions_immutable raises with ERRCODE 23514, and
// the caller must see INVALID_STATE_TRANSITION rather than a database error.
func TestUpdateVersionDraftMapsTheImmutabilityTrigger(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{Code: "23514", Message: "cannot update a published content version"}
	repo := repoFailingWith(pgErr)

	_, err := repo.UpdateVersionDraft(
		context.Background(), uuid.New(), "vocab_word", []byte("{}"), domain.CEFRB1, nil, domain.StatusDraft,
	)

	// WithInternal returns a fresh *apperr.Error rather than wrapping the
	// sentinel, so the assertion is on the code the API contract names.
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperr.Error", err)
	}
	if appErr.Code != "INVALID_STATE_TRANSITION" {
		t.Fatalf("error code = %q, want INVALID_STATE_TRANSITION", appErr.Code)
	}
	if appErr.Kind != apperr.Conflict {
		t.Errorf("error kind = %v, want Conflict (409)", appErr.Kind)
	}
}

// TestUnrecognisedErrorsAreWrappedNotSwallowed proves the mapping is a
// translation and not a catch-all: an unrelated failure must keep its identity
// so it can be logged and alerted on.
func TestUnrecognisedErrorsAreWrappedNotSwallowed(t *testing.T) {
	t.Parallel()

	boom := errors.New("connection reset by peer")
	repo := repoFailingWith(boom)

	_, err := repo.GetItemByID(context.Background(), uuid.New())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap the underlying failure", err)
	}
	if errors.Is(err, domain.ErrItemNotFound) {
		t.Error("an unrelated failure was reported as a missing item")
	}
}

// TestListQueriesPropagateTheirError covers the `:many` path, which fails at
// Query rather than Scan.
func TestListQueriesPropagateTheirError(t *testing.T) {
	t.Parallel()

	boom := errors.New("query failed")
	repo := repoFailingWith(boom)
	ctx := context.Background()

	if _, err := repo.ListVersionsByItemID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListVersionsByItemID error = %v, want the underlying failure", err)
	}
	if _, err := repo.GetManyVersionsByIDs(ctx, []uuid.UUID{uuid.New()}); !errors.Is(err, boom) {
		t.Errorf("GetManyVersionsByIDs error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListItemsByOwner(ctx, uuid.New(), 10); !errors.Is(err, boom) {
		t.Errorf("ListItemsByOwner error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListTagsForContentItem(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListTagsForContentItem error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListTagsForContentItems(ctx, []uuid.UUID{uuid.New()}); !errors.Is(err, boom) {
		t.Errorf("ListTagsForContentItems error = %v, want the underlying failure", err)
	}
	if _, err := repo.ListReviewsForVersion(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ListReviewsForVersion error = %v, want the underlying failure", err)
	}
	if _, err := repo.GetMediaAssetsByObjectKeys(ctx, []string{"audio/a.mp3"}); !errors.Is(err, boom) {
		t.Errorf("GetMediaAssetsByObjectKeys error = %v, want the underlying failure", err)
	}
	if _, err := repo.BrowsePublishedVersions(ctx, nil, nil, 10, 0); !errors.Is(err, boom) {
		t.Errorf("BrowsePublishedVersions error = %v, want the underlying failure", err)
	}
}

// TestExecQueriesPropagateTheirError covers the `:exec` path.
func TestExecQueriesPropagateTheirError(t *testing.T) {
	t.Parallel()

	boom := errors.New("exec failed")
	repo := repoFailingWith(boom)
	ctx := context.Background()

	if err := repo.DeleteItem(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("DeleteItem error = %v, want the underlying failure", err)
	}
	if err := repo.AddContentTag(ctx, uuid.New(), uuid.New()); !errors.Is(err, boom) {
		t.Errorf("AddContentTag error = %v, want the underlying failure", err)
	}
	if err := repo.ClearTagsForContentItem(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("ClearTagsForContentItem error = %v, want the underlying failure", err)
	}
}

// TestSingleRowQueriesPropagateTheirError covers the remaining `:one` methods
// whose failure is not translated into a domain sentinel.
func TestSingleRowQueriesPropagateTheirError(t *testing.T) {
	t.Parallel()

	boom := errors.New("scan failed")
	repo := repoFailingWith(boom)
	ctx := context.Background()
	checksum := "abc123"
	var duration int32 = 1500

	if _, err := repo.GetLatestVersionNumberByItemID(ctx, uuid.New()); !errors.Is(err, boom) {
		t.Errorf("GetLatestVersionNumberByItemID error = %v, want the underlying failure", err)
	}
	if _, err := repo.CountPublishedVersions(ctx, nil, nil); !errors.Is(err, boom) {
		t.Errorf("CountPublishedVersions error = %v, want the underlying failure", err)
	}
	if _, err := repo.GetMediaAssetByObjectKey(ctx, "audio/a.mp3"); !errors.Is(err, boom) {
		t.Errorf("GetMediaAssetByObjectKey error = %v, want the underlying failure", err)
	}
	if _, err := repo.CreateMediaAsset(
		ctx, uuid.New(), "audio/a.mp3", "audio", &duration, &checksum, domain.MediaStatusPending, nil, nil,
	); !errors.Is(err, boom) {
		t.Errorf("CreateMediaAsset error = %v, want the underlying failure", err)
	}
	_, err := repo.UpdateMediaAssetStatus(ctx, uuid.New(), domain.MediaStatusReady, nil, nil, nil, nil)
	if !errors.Is(err, boom) {
		t.Errorf("UpdateMediaAssetStatus error = %v, want the underlying failure", err)
	}
	if _, err := repo.CreateReview(
		ctx, uuid.New(), uuid.New(), uuid.New(), domain.ReviewDecisionApproved, nil,
	); !errors.Is(err, boom) {
		t.Errorf("CreateReview error = %v, want the underlying failure", err)
	}
	if _, err := repo.GetTaxonomyByNamespaceCode(ctx, "topic", "environment"); !errors.Is(err, boom) {
		t.Errorf("GetTaxonomyByNamespaceCode error = %v, want the underlying failure", err)
	}
	if _, err := repo.CreateVersion(
		ctx, uuid.New(), uuid.New(), 1, "vocab_word", []byte("{}"), domain.CEFRB1, domain.StatusDraft, nil, nil,
	); !errors.Is(err, boom) {
		t.Errorf("CreateVersion error = %v, want the underlying failure", err)
	}
}
