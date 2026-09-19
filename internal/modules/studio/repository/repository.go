package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/studio/sqlc"
	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository defines data access operations for the studio module.
type Repository interface {
	GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*domain.CreatorProfile, error)
	UpsertCreatorProfile(ctx context.Context, userID uuid.UUID, bio, headline string) (*domain.CreatorProfile, error)
	SetPayoutEligible(ctx context.Context, userID uuid.UUID, eligible bool) (*domain.CreatorProfile, error)

	GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error)
	UpsertPayoutAccount(ctx context.Context, creatorID uuid.UUID, bankCode, accountNumber, accountHolderName string, isDefault bool) (*domain.PayoutAccount, error)

	CreateCourseDraft(ctx context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error)
	GetCourseDraftByID(ctx context.Context, id uuid.UUID) (*domain.CourseDraft, error)
	GetCourseDraftByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*domain.CourseDraft, error)
	ListCourseDraftsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.CourseDraft, int64, error)
	UpdateCourseDraft(ctx context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error)
	UpdateCourseDraftStatus(ctx context.Context, id uuid.UUID, status string) (*domain.CourseDraft, error)

	CreateSubmission(ctx context.Context, sub *domain.Submission) (*domain.Submission, error)
	GetSubmissionByID(ctx context.Context, id uuid.UUID) (*domain.Submission, error)
	GetLatestSubmissionByDraftID(ctx context.Context, draftID uuid.UUID) (*domain.Submission, error)
	ListSubmissionsByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.Submission, int64, error)
	UpdateSubmissionVerification(ctx context.Context, id uuid.UUID, status string, report []byte, feedback *string) (*domain.Submission, error)
	UpdateSubmissionReview(ctx context.Context, id uuid.UUID, status string, reviewerID uuid.UUID, feedback *string) (*domain.Submission, error)
	ListPendingVerificationSubmissions(ctx context.Context, limit int32) ([]*domain.Submission, error)
}

type pgRepository struct {
	q *sqlc.Queries
}

// NewRepository creates a new studio repository.
func NewRepository(db dbx.Querier) Repository {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	}
	return &pgRepository{
		q: q,
	}
}

func (r *pgRepository) GetCreatorProfile(ctx context.Context, userID uuid.UUID) (*domain.CreatorProfile, error) {
	row, err := r.q.GetCreatorProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProfileNotFound
		}
		return nil, err
	}
	return &domain.CreatorProfile{
		UserID:         row.UserID,
		Bio:            row.Bio,
		Headline:       row.Headline,
		PayoutEligible: row.PayoutEligible,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func (r *pgRepository) UpsertCreatorProfile(ctx context.Context, userID uuid.UUID, bio, headline string) (*domain.CreatorProfile, error) {
	row, err := r.q.UpsertCreatorProfile(ctx, sqlc.UpsertCreatorProfileParams{
		UserID:   userID,
		Bio:      bio,
		Headline: headline,
	})
	if err != nil {
		return nil, err
	}
	return &domain.CreatorProfile{
		UserID:         row.UserID,
		Bio:            row.Bio,
		Headline:       row.Headline,
		PayoutEligible: row.PayoutEligible,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func (r *pgRepository) SetPayoutEligible(ctx context.Context, userID uuid.UUID, eligible bool) (*domain.CreatorProfile, error) {
	row, err := r.q.SetPayoutEligible(ctx, sqlc.SetPayoutEligibleParams{
		UserID:         userID,
		PayoutEligible: eligible,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProfileNotFound
		}
		return nil, err
	}
	return &domain.CreatorProfile{
		UserID:         row.UserID,
		Bio:            row.Bio,
		Headline:       row.Headline,
		PayoutEligible: row.PayoutEligible,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func (r *pgRepository) GetPayoutAccount(ctx context.Context, creatorID uuid.UUID) (*domain.PayoutAccount, error) {
	row, err := r.q.GetPayoutAccountByCreatorID(ctx, creatorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.PayoutAccount{
		ID:                row.ID,
		CreatorID:         row.CreatorID,
		BankCode:          row.BankCode,
		AccountNumber:     row.AccountNumber,
		AccountHolderName: row.AccountHolderName,
		IsDefault:         row.IsDefault,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func (r *pgRepository) UpsertPayoutAccount(ctx context.Context, creatorID uuid.UUID, bankCode, accountNumber, accountHolderName string, isDefault bool) (*domain.PayoutAccount, error) {
	row, err := r.q.UpsertPayoutAccount(ctx, sqlc.UpsertPayoutAccountParams{
		CreatorID:         creatorID,
		BankCode:          bankCode,
		AccountNumber:     accountNumber,
		AccountHolderName: accountHolderName,
		IsDefault:         isDefault,
	})
	if err != nil {
		return nil, err
	}
	return &domain.PayoutAccount{
		ID:                row.ID,
		CreatorID:         row.CreatorID,
		BankCode:          row.BankCode,
		AccountNumber:     row.AccountNumber,
		AccountHolderName: row.AccountHolderName,
		IsDefault:         row.IsDefault,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func (r *pgRepository) CreateCourseDraft(ctx context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error) {
	row, err := r.q.CreateCourseDraft(ctx, sqlc.CreateCourseDraftParams{
		OwnerID:         draft.OwnerID,
		Title:           draft.Title,
		Slug:            draft.Slug,
		Description:     draft.Description,
		CefrLevel:       draft.CEFRLevel,
		TopicTaxonomyID: draft.TopicTaxonomyID,
		PriceVnd:        draft.PriceVND,
		Structure:       draft.Structure,
	})
	if err != nil {
		return nil, err
	}
	return toDomainDraft(row), nil
}

func (r *pgRepository) GetCourseDraftByID(ctx context.Context, id uuid.UUID) (*domain.CourseDraft, error) {
	row, err := r.q.GetCourseDraftByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDraftNotFound
		}
		return nil, err
	}
	return toDomainDraft(row), nil
}

func (r *pgRepository) GetCourseDraftByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*domain.CourseDraft, error) {
	row, err := r.q.GetCourseDraftByOwnerAndSlug(ctx, sqlc.GetCourseDraftByOwnerAndSlugParams{
		OwnerID: ownerID,
		Slug:    slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDraftNotFound
		}
		return nil, err
	}
	return toDomainDraft(row), nil
}

func (r *pgRepository) ListCourseDraftsByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.CourseDraft, int64, error) {
	rows, err := r.q.ListCourseDraftsByOwner(ctx, sqlc.ListCourseDraftsByOwnerParams{
		OwnerID: ownerID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountCourseDraftsByOwner(ctx, ownerID)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*domain.CourseDraft, len(rows))
	for i, row := range rows {
		items[i] = toDomainDraft(row)
	}
	return items, total, nil
}

func (r *pgRepository) UpdateCourseDraft(ctx context.Context, draft *domain.CourseDraft) (*domain.CourseDraft, error) {
	row, err := r.q.UpdateCourseDraft(ctx, sqlc.UpdateCourseDraftParams{
		ID:              draft.ID,
		Title:           draft.Title,
		Slug:            draft.Slug,
		Description:     draft.Description,
		CefrLevel:       draft.CEFRLevel,
		TopicTaxonomyID: draft.TopicTaxonomyID,
		PriceVnd:        draft.PriceVND,
		Structure:       draft.Structure,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDraftNotFound
		}
		return nil, err
	}
	return toDomainDraft(row), nil
}

func (r *pgRepository) UpdateCourseDraftStatus(ctx context.Context, id uuid.UUID, status string) (*domain.CourseDraft, error) {
	row, err := r.q.UpdateCourseDraftStatus(ctx, sqlc.UpdateCourseDraftStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDraftNotFound
		}
		return nil, err
	}
	return toDomainDraft(row), nil
}

func (r *pgRepository) CreateSubmission(ctx context.Context, sub *domain.Submission) (*domain.Submission, error) {
	row, err := r.q.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		DraftID:     sub.DraftID,
		Version:     int32(sub.Version),
		Status:      sub.Status,
		SubmittedBy: sub.SubmittedBy,
	})
	if err != nil {
		return nil, err
	}
	return toDomainSubmission(row), nil
}

func (r *pgRepository) GetSubmissionByID(ctx context.Context, id uuid.UUID) (*domain.Submission, error) {
	row, err := r.q.GetSubmissionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSubmissionNotFound
		}
		return nil, err
	}
	return toDomainSubmission(row), nil
}

func (r *pgRepository) GetLatestSubmissionByDraftID(ctx context.Context, draftID uuid.UUID) (*domain.Submission, error) {
	row, err := r.q.GetLatestSubmissionByDraftID(ctx, draftID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainSubmission(row), nil
}

func (r *pgRepository) ListSubmissionsByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.Submission, int64, error) {
	rows, err := r.q.ListSubmissionsByStatus(ctx, sqlc.ListSubmissionsByStatusParams{
		Status: status,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountSubmissionsByStatus(ctx, status)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*domain.Submission, len(rows))
	for i, row := range rows {
		items[i] = toDomainSubmission(row)
	}
	return items, total, nil
}

func (r *pgRepository) UpdateSubmissionVerification(ctx context.Context, id uuid.UUID, status string, report []byte, feedback *string) (*domain.Submission, error) {
	row, err := r.q.UpdateSubmissionVerification(ctx, sqlc.UpdateSubmissionVerificationParams{
		ID:                 id,
		Status:             status,
		VerificationReport: report,
		Feedback:           feedback,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSubmissionNotFound
		}
		return nil, err
	}
	return toDomainSubmission(row), nil
}

func (r *pgRepository) UpdateSubmissionReview(ctx context.Context, id uuid.UUID, status string, reviewerID uuid.UUID, feedback *string) (*domain.Submission, error) {
	row, err := r.q.UpdateSubmissionReview(ctx, sqlc.UpdateSubmissionReviewParams{
		ID:         id,
		Status:     status,
		ReviewerID: &reviewerID,
		Feedback:   feedback,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSubmissionNotFound
		}
		return nil, err
	}
	return toDomainSubmission(row), nil
}

func (r *pgRepository) ListPendingVerificationSubmissions(ctx context.Context, limit int32) ([]*domain.Submission, error) {
	rows, err := r.q.ListPendingVerificationSubmissions(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Submission, len(rows))
	for i, row := range rows {
		items[i] = toDomainSubmission(row)
	}
	return items, nil
}

func toDomainDraft(row sqlc.StudioCourseDraft) *domain.CourseDraft {
	return &domain.CourseDraft{
		ID:              row.ID,
		OwnerID:         row.OwnerID,
		Title:           row.Title,
		Slug:            row.Slug,
		Description:     row.Description,
		CEFRLevel:       row.CefrLevel,
		TopicTaxonomyID: row.TopicTaxonomyID,
		PriceVND:        row.PriceVnd,
		Status:          row.Status,
		Structure:       row.Structure,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func toDomainSubmission(row sqlc.StudioSubmission) *domain.Submission {
	return &domain.Submission{
		ID:                 row.ID,
		DraftID:            row.DraftID,
		Version:            int(row.Version),
		Status:             row.Status,
		SubmittedBy:        row.SubmittedBy,
		ReviewerID:         row.ReviewerID,
		Feedback:           row.Feedback,
		VerificationReport: row.VerificationReport,
		SubmittedAt:        row.SubmittedAt,
		ReviewedAt:         row.ReviewedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}
