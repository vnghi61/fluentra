package service

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

// minDailySample is the floor under the daily share: a small run still gets a
// person's eyes on it.
const minDailySample = 5

// sampleShare is the fraction of a day's auto-published items drawn for review.
const sampleShare = 0.02

// SampleAutoPublished draws a day's random sample of auto-published versions
// into the sample queue (WO 22 Stage A.5, D22-3).
//
// At least five, at most everything, and never more than a day's total. Running
// it twice for the same day writes no duplicate: the sample row is keyed on the
// version.
func (s *Service) SampleAutoPublished(ctx context.Context, day time.Time) (int, error) {
	total, err := s.repo.CountAutoPublishedOn(ctx, day)
	if err != nil {
		return 0, err
	}
	size := sampleSize(total)
	if size == 0 {
		return 0, nil
	}
	ids, err := s.repo.ListAutoPublishedOn(ctx, day, size)
	if err != nil {
		return 0, err
	}
	sampled := 0
	for _, id := range ids {
		if err := s.repo.InsertReviewSample(ctx, id, "sample", day); err != nil {
			return sampled, err
		}
		sampled++
	}
	return sampled, nil
}

// sampleSize is the count drawn from a day's total: 2 %, at least five, and
// never more than the total itself.
func sampleSize(total int64) int {
	if total <= 0 {
		return 0
	}
	size := int(math.Ceil(float64(total) * sampleShare))
	if size < minDailySample {
		size = minDailySample
	}
	if int64(size) > total {
		size = int(total)
	}
	return size
}

// ReviewSamples lists the samples still waiting for a person.
func (s *Service) ReviewSamples(
	ctx context.Context, limit, offset int,
) ([]domain.ReviewSample, int64, error) {
	samples, err := s.repo.ListOpenReviewSamples(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountOpenReviewSamples(ctx)
	if err != nil {
		return nil, 0, err
	}
	return samples, count, nil
}

// KeepReviewSample records that a person checked a sample and it stands.
func (s *Service) KeepReviewSample(
	ctx context.Context, reviewerID, versionID uuid.UUID, note *string,
) error {
	return s.repo.DecideReviewSample(ctx, versionID, domain.SampleKept, note, reviewerID)
}

// RejectReviewSample unpublishes a sampled item: the item is archived, which
// emits content.archived and stops the question being drawn, and the sample is
// marked rejected (D22-3, BR-EXAM-12).
//
// Archiving never rewrites a stored composition: an attempt already built from
// this item keeps it.
func (s *Service) RejectReviewSample(
	ctx context.Context, reviewerID, versionID uuid.UUID, note *string,
) error {
	version, err := s.repo.GetVersionByID(ctx, versionID)
	if err != nil {
		return err
	}
	if err := s.repo.DecideReviewSample(ctx, versionID, domain.SampleRejected, note, reviewerID); err != nil {
		return err
	}
	if _, err := s.Archive(ctx, reviewerID, version.ItemID); err != nil {
		return err
	}
	return nil
}

// pullAutoPublishedForReview queues an auto-published version a learner
// reported, and unpublishes it (WO 22 Stage A.5).
func (s *Service) pullAutoPublishedForReview(
	ctx context.Context, userID uuid.UUID, version domain.Version,
) error {
	auto, err := s.repo.IsAutoPublishedVersion(ctx, version.ID)
	if err != nil {
		return err
	}
	if !auto {
		return nil
	}
	if err := s.repo.InsertReviewSample(ctx, version.ID, "report", s.clock.Now().UTC()); err != nil {
		return err
	}
	if _, err := s.Archive(ctx, userID, version.ItemID); err != nil {
		return err
	}
	return nil
}
