package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// machineReviewComment is what a generated version's approval row says.
//
// Recorded rather than skipped: a published version with no approval is a state
// the authoring API has never produced, and the first person to meet one would
// read it as a bug in the review workflow rather than as generated content.
const machineReviewComment = "Approved automatically: machine-authored content."

// EnsurePublished implements contract.Author.
//
// One call rather than the four-step review state machine, because the caller is
// a scheduled job and not a person: `CreateItem` → `SubmitForReview` → `Review`
// → `Publish` models decisions somebody makes, and a machine walking that path
// is a machine approving its own work across four transactions it does not own.
//
// The three cases, in the order they are checked:
//
//  1. No item at this slug — create the item, version 1, publish, link.
//  2. An item whose current published version has the same body — do nothing and
//     return it. This is the common case: the generator re-runs on a schedule,
//     and re-writing identical content every hour would churn the table and
//     invalidate every cache keyed on the version id.
//  3. An item whose body has changed — a new version, published, linked. The old
//     version stays, because content versions are immutable and something may
//     already point at it.
func (s *Service) EnsurePublished(ctx context.Context, spec contract.AuthorSpec) (uuid.UUID, error) {
	if err := validateAuthorSpec(spec); err != nil {
		return uuid.Nil, err
	}

	var versionID uuid.UUID
	err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		item, err := repo.GetItemBySlug(txCtx, spec.Slug)
		switch {
		case err == nil:
			// Cases 2 and 3.
			versionID, err = s.republish(txCtx, repo, item, spec)
			return err
		case errors.Is(err, domain.ErrItemNotFound), errors.Is(err, pgx.ErrNoRows):
			// Case 1.
			versionID, err = s.authorFirstVersion(txCtx, repo, spec)
			return err
		default:
			return err
		}
	})
	if err != nil {
		return uuid.Nil, err
	}
	return versionID, nil
}

// authorFirstVersion creates the item and its published version 1.
func (s *Service) authorFirstVersion(
	ctx context.Context, repo Repository, spec contract.AuthorSpec,
) (uuid.UUID, error) {
	item, err := repo.CreateItem(
		ctx, s.newID(), spec.Kind, spec.Slug, domain.StatusPublished, spec.AuthorID,
	)
	if err != nil {
		return uuid.Nil, err
	}
	return s.publishVersion(ctx, repo, item.ID, 1, spec)
}

// republish returns the current version when the body is unchanged, and writes
// the next version when it is not.
func (s *Service) republish(
	ctx context.Context, repo Repository, item domain.Item, spec contract.AuthorSpec,
) (uuid.UUID, error) {
	if item.CurrentVersionID != nil && *item.CurrentVersionID != uuid.Nil {
		current, err := repo.GetVersionByID(ctx, *item.CurrentVersionID)
		if err == nil && sameBody(current, spec) {
			return current.ID, nil
		}
	}

	latest, err := repo.GetLatestVersionNumberByItemID(ctx, item.ID)
	if err != nil {
		return uuid.Nil, err
	}
	return s.publishVersion(ctx, repo, item.ID, latest+1, spec)
}

// publishVersion writes one published version, its approval row, and the link
// from the item to it.
func (s *Service) publishVersion(
	ctx context.Context, repo Repository, itemID uuid.UUID, number int, spec contract.AuthorSpec,
) (uuid.UUID, error) {
	now := s.clock.Now()
	version, err := repo.CreateVersion(
		ctx, s.newID(), itemID, number, spec.Kind, spec.Body,
		spec.CEFRLevel, domain.StatusPublished, nil, &now,
	)
	if err != nil {
		return uuid.Nil, err
	}

	comment := machineReviewComment
	if _, err := repo.CreateReview(
		ctx, s.newID(), version.ID, spec.AuthorID, domain.ReviewDecisionApproved, &comment,
	); err != nil {
		return uuid.Nil, err
	}

	if _, err := repo.UpdateItemStatus(ctx, itemID, domain.StatusPublished); err != nil {
		return uuid.Nil, err
	}
	if _, err := repo.UpdateItemCurrentVersion(ctx, itemID, &version.ID); err != nil {
		return uuid.Nil, err
	}
	return version.ID, nil
}

// sameBody reports whether a stored version already says what the spec says.
//
// Compared as decoded JSON, not as bytes: the stored copy has been through
// Postgres's jsonb, which reorders keys and drops insignificant whitespace, so a
// byte comparison reports every version as changed and the generator rewrites
// the whole catalogue on every run.
func sameBody(version domain.Version, spec contract.AuthorSpec) bool {
	if version.Kind != spec.Kind || version.CEFRLevel != spec.CEFRLevel {
		return false
	}
	if version.Status != domain.StatusPublished {
		return false
	}

	var stored, wanted any
	if err := json.Unmarshal(version.Body, &stored); err != nil {
		return false
	}
	if err := json.Unmarshal(spec.Body, &wanted); err != nil {
		return false
	}
	// Re-encoding both through the same marshaller gives them the same key
	// order, which is what makes the comparison meaningful.
	storedJSON, err := json.Marshal(stored)
	if err != nil {
		return false
	}
	wantedJSON, err := json.Marshal(wanted)
	if err != nil {
		return false
	}
	return bytes.Equal(storedJSON, wantedJSON)
}

func validateAuthorSpec(spec contract.AuthorSpec) error {
	invalid := func(message string) error {
		return apperr.New(apperr.Validation, "CONTENT_AUTHOR_SPEC_INVALID", message)
	}
	switch {
	case strings.TrimSpace(spec.Slug) == "":
		return invalid("Authored content needs a slug.")
	case strings.TrimSpace(spec.Kind) == "":
		return invalid("Authored content needs a kind.")
	case spec.AuthorID == uuid.Nil:
		return invalid("Authored content needs an author.")
	case len(spec.Body) == 0:
		return invalid("Authored content needs a body.")
	}
	if !json.Valid(spec.Body) {
		return invalid(fmt.Sprintf("The body for %q is not valid JSON.", spec.Slug))
	}
	return nil
}

var _ contract.Author = (*Service)(nil)
