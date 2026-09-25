package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
			if err != nil {
				return err
			}
			return s.attachTags(txCtx, repo, item.ID, spec.Tags)
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

// EnsureDraft implements contract.Author for draft content.
//
// The door Stages D and F use: content that a person must review enters as a
// draft. Idempotent on the slug: an existing draft with the same body writes
// nothing and returns it. An existing published item with a different body gets
// a new draft version rather than altering published content (rule BR-CONTENT-01).
func (s *Service) EnsureDraft(ctx context.Context, spec contract.AuthorSpec) (uuid.UUID, error) {
	if err := validateAuthorSpec(spec); err != nil {
		return uuid.Nil, err
	}

	var versionID uuid.UUID
	err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		item, err := repo.GetItemBySlug(txCtx, spec.Slug)
		switch {
		case err == nil:
			// Check if current version is already a draft with the same body.
			if item.CurrentVersionID != nil && *item.CurrentVersionID != uuid.Nil {
				current, err := repo.GetVersionByID(txCtx, *item.CurrentVersionID)
				if err == nil && sameDraftBody(current, spec) {
					versionID = current.ID
					return s.attachTags(txCtx, repo, item.ID, spec.Tags)
				}
			}

			latest, err := repo.GetLatestVersionNumberByItemID(txCtx, item.ID)
			if err != nil {
				return err
			}
			versionID, err = s.draftVersion(txCtx, repo, item.ID, latest+1, spec)
			if err != nil {
				return err
			}
			return s.attachTags(txCtx, repo, item.ID, spec.Tags)

		case errors.Is(err, domain.ErrItemNotFound), errors.Is(err, pgx.ErrNoRows):
			item, err := repo.CreateItem(
				txCtx, s.newID(), spec.Kind, spec.Slug, domain.StatusDraft, spec.AuthorID,
			)
			if err != nil {
				return err
			}
			versionID, err = s.draftVersion(txCtx, repo, item.ID, 1, spec)
			if err != nil {
				return err
			}
			return s.attachTags(txCtx, repo, item.ID, spec.Tags)

		default:
			return err
		}
	})
	if err != nil {
		return uuid.Nil, err
	}
	return versionID, nil
}

// ApproveVerified implements contract.Author.
//
// The door WO 22 Stage A opens: a draft an independent verifier confirmed is
// published without a person. It walks the version draft → in_review → approved
// → published with the same state machine and events as the human path, and
// records a content_reviews row whose reviewer is the item's owner — the admin
// the generator authored under. There is no anonymous approval: reviewer_id is
// NOT NULL and references core.users.
//
// A verification that is not confirmed is refused here rather than published;
// it stays in the review queue for a person. Publishing, not judging, is this
// door's job.
func (s *Service) ApproveVerified(
	ctx context.Context, versionID uuid.UUID, verification contract.Verification,
) error {
	if versionID == uuid.Nil {
		return apperr.New(apperr.Validation, "CONTENT_VERSION_REQUIRED", "A content version is required.")
	}
	if !verification.Confirmed {
		return apperr.New(apperr.Conflict, "CONTENT_VERIFICATION_NOT_CONFIRMED",
			"The independent verifier did not confirm this version; it stays for a person.")
	}

	return dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		version, err := repo.GetVersionByID(txCtx, versionID)
		if err != nil {
			return err
		}
		item, err := repo.GetItemByID(txCtx, version.ItemID)
		if err != nil {
			return err
		}
		// Idempotent: a verifier that runs twice publishes once.
		if version.Status == domain.StatusPublished {
			return nil
		}

		version, err = markVersionVerified(txCtx, repo, version, verification)
		if err != nil {
			return err
		}

		approved, err := s.advanceToApproved(txCtx, repo, item, version)
		if err != nil {
			return err
		}
		if _, err := s.finalizePublished(txCtx, tx, repo, item, approved); err != nil {
			return err
		}

		comment := fmt.Sprintf("Approved by independent verifier %s at %s.",
			verifierName(verification.Model), verification.CheckedAt.UTC().Format(time.RFC3339))
		if _, err := repo.CreateReview(
			txCtx, s.newID(), version.ID, item.OwnerID, domain.ReviewDecisionApproved, &comment,
		); err != nil {
			return err
		}
		return nil
	})
}

// ErrBatchEscalated reports a batch the verifier did not confirm whole, or that
// a publication gate refused: it stays as drafts for a person.
var ErrBatchEscalated = apperr.New(apperr.Conflict, "CONTENT_BATCH_ESCALATED",
	"The batch was not confirmed whole; it stays for a person to review.")

// ApproveVerifiedBatch implements contract.VerifiedBatchPublisher (D22-13).
//
// Every draft of the batch must carry a confirmed verification; then all of
// them are published in one transaction, topics after the items their gate
// needs, each with a content_reviews row naming the verifier. Otherwise nothing
// changes and the error says which version held the batch back.
func (s *Service) ApproveVerifiedBatch(ctx context.Context, batch string) (int, error) {
	if strings.TrimSpace(batch) == "" {
		return 0, apperr.New(apperr.Validation, "CONTENT_BATCH_REQUIRED", "A batch is required.")
	}
	published := 0
	err := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		ids, err := repo.ListReviewBatchVersionIDs(txCtx, batch)
		if err != nil {
			return err
		}
		ids, err = topicsLast(txCtx, repo, ids)
		if err != nil {
			return err
		}
		versions := make([]domain.Version, 0, len(ids))
		for _, id := range ids {
			version, err := repo.GetVersionByID(txCtx, id)
			if err != nil {
				return err
			}
			if verification, ok := recordedVerification(version.Body); !ok || !verification.Confirmed {
				return ErrBatchEscalated.WithInternal(fmt.Sprintf("version %s is not confirmed", id))
			}
			versions = append(versions, version)
		}
		for _, version := range versions {
			if err := s.publishVerifiedVersion(txCtx, tx, repo, version); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	if err != nil {
		published = 0
		if isPublicationGate(err) {
			return 0, ErrBatchEscalated.WithInternal(err.Error())
		}
		return 0, err
	}
	return published, nil
}

// publishVerifiedVersion walks one confirmed draft to published and records the
// verifier's approval, reviewer the item's owner.
func (s *Service) publishVerifiedVersion(
	ctx context.Context, tx pgx.Tx, repo Repository, version domain.Version,
) error {
	item, err := repo.GetItemByID(ctx, version.ItemID)
	if err != nil {
		return err
	}
	approved, err := s.advanceToApproved(ctx, repo, item, version)
	if err != nil {
		return err
	}
	if _, err := s.finalizePublished(ctx, tx, repo, item, approved); err != nil {
		return err
	}
	verification, _ := recordedVerification(version.Body)
	comment := fmt.Sprintf("Approved by independent verifier %s at %s, with its whole batch.",
		verifierName(verification.Model), verification.CheckedAt.UTC().Format(time.RFC3339))
	_, err = repo.CreateReview(ctx, s.newID(), version.ID, item.OwnerID, domain.ReviewDecisionApproved, &comment)
	return err
}

// recordedVerification reads `_provenance.verification` back from a body.
func recordedVerification(body json.RawMessage) (contract.Verification, bool) {
	var decoded struct {
		Provenance struct {
			Verification *struct {
				Model     string `json:"model"`
				Verdict   string `json:"verdict"`
				Reason    string `json:"reason"`
				CheckedAt string `json:"checked_at"`
			} `json:"verification"`
		} `json:"_provenance"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil || decoded.Provenance.Verification == nil {
		return contract.Verification{}, false
	}
	mark := decoded.Provenance.Verification
	checkedAt, _ := time.Parse(time.RFC3339, mark.CheckedAt)
	return contract.Verification{
		Confirmed: mark.Verdict == "confirmed",
		Model:     mark.Model,
		Reason:    mark.Reason,
		CheckedAt: checkedAt,
	}, true
}

// isPublicationGate reports a refusal by a publication gate rather than a fault.
func isPublicationGate(err error) bool {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		return false
	}
	return appErr.Code == domain.ErrFoundationIncomplete.Code || appErr.Code == domain.ErrMediaNotReady.Code ||
		appErr.Code == ErrBatchEscalated.Code
}

// ApproveRecorded implements contract.RecordedApprover: a draft a person
// approved on the machine that generated it is published, with a review row by
// the item's owner saying the approval was recorded in the fixture.
func (s *Service) ApproveRecorded(ctx context.Context, versionID uuid.UUID, approvedAt time.Time) error {
	if versionID == uuid.Nil {
		return apperr.New(apperr.Validation, "CONTENT_VERSION_REQUIRED", "A content version is required.")
	}
	return dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		version, err := repo.GetVersionByID(txCtx, versionID)
		if err != nil {
			return err
		}
		if version.Status == domain.StatusPublished {
			return nil
		}
		item, err := repo.GetItemByID(txCtx, version.ItemID)
		if err != nil {
			return err
		}
		approved, err := s.advanceToApproved(txCtx, repo, item, version)
		if err != nil {
			return err
		}
		if _, err := s.finalizePublished(txCtx, tx, repo, item, approved); err != nil {
			return err
		}
		comment := "Approved by a person"
		if !approvedAt.IsZero() {
			comment += " on " + approvedAt.UTC().Format("2006-01-02")
		}
		comment += "; loaded from the frozen fixture."
		_, err = repo.CreateReview(txCtx, s.newID(), version.ID, item.OwnerID, domain.ReviewDecisionApproved, &comment)
		return err
	})
}

// RecordVerification implements contract.VerificationRecorder.
//
// It writes the verifier's outcome into a version that is still a draft, so a
// doubt waits in its batch with its reason. A published version is immutable and
// is refused rather than silently ignored.
func (s *Service) RecordVerification(
	ctx context.Context, versionID uuid.UUID, verification contract.Verification,
) error {
	if versionID == uuid.Nil {
		return apperr.New(apperr.Validation, "CONTENT_VERSION_REQUIRED", "A content version is required.")
	}
	version, err := s.repo.GetVersionByID(ctx, versionID)
	if err != nil {
		return err
	}
	if version.Status == domain.StatusPublished {
		return apperr.New(apperr.Conflict, "CONTENT_VERSION_PUBLISHED",
			"A published version is immutable; its verification cannot be rewritten.")
	}
	_, err = markVersionVerified(ctx, s.repo, version, verification)
	return err
}

// markVersionVerified merges the verifier's marking into a version's body and
// writes it back with its status unchanged.
func markVersionVerified(
	ctx context.Context, repo Repository, version domain.Version, verification contract.Verification,
) (domain.Version, error) {
	body, err := mergeVerification(version.Body, verification)
	if err != nil {
		return domain.Version{}, err
	}
	return repo.UpdateVersionDraft(
		ctx, version.ID, version.Kind, body, version.CEFRLevel, version.MediaRefs, version.Status,
	)
}

// mergeVerification writes `_provenance.verification` into a body.
func mergeVerification(body json.RawMessage, verification contract.Verification) (json.RawMessage, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode body for verification marking: %w", err)
	}
	prov, _ := decoded["_provenance"].(map[string]any)
	if prov == nil {
		prov = map[string]any{}
	}
	mark := map[string]any{
		"model":      verifierName(verification.Model),
		"verdict":    verdictString(verification.Confirmed),
		"checked_at": verification.CheckedAt.UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(verification.Reason) != "" {
		mark["reason"] = verification.Reason
	}
	prov["verification"] = mark
	decoded["_provenance"] = prov
	return json.Marshal(decoded)
}

// verdictString is the marking's verdict for a verification.
func verdictString(confirmed bool) string {
	if confirmed {
		return "confirmed"
	}
	return "doubt"
}

// advanceToApproved moves a draft or in-review version to approved, running the
// same publication gates a person's publish would.
func (s *Service) advanceToApproved(
	ctx context.Context, repo Repository, item domain.Item, version domain.Version,
) (domain.Version, error) {
	if err := s.verifyMediaAssetsReady(ctx, repo, version.MediaRefs); err != nil {
		return domain.Version{}, err
	}
	if err := verifyFoundationComplete(ctx, repo, item.Kind, item.ID); err != nil {
		return domain.Version{}, err
	}

	steps, err := stepsToApproved(version.Status)
	if err != nil {
		return domain.Version{}, err
	}
	current := version
	for _, next := range steps {
		if err := domain.ValidateTransition(current.Status, next); err != nil {
			return domain.Version{}, err
		}
		updated, err := repo.UpdateVersionDraft(
			ctx, current.ID, current.Kind, []byte(current.Body), current.CEFRLevel, current.MediaRefs, next,
		)
		if err != nil {
			return domain.Version{}, err
		}
		current = updated
	}
	if _, err := repo.UpdateItemStatus(ctx, item.ID, domain.StatusApproved); err != nil {
		return domain.Version{}, err
	}
	if err := domain.ValidateTransition(current.Status, domain.StatusPublished); err != nil {
		return domain.Version{}, err
	}
	return current, nil
}

// stepsToApproved is the remaining transitions from a version's status to
// approved. A version already approved needs none.
func stepsToApproved(from domain.AuthoringStatus) ([]domain.AuthoringStatus, error) {
	switch from {
	case domain.StatusDraft:
		return []domain.AuthoringStatus{domain.StatusInReview, domain.StatusApproved}, nil
	case domain.StatusInReview:
		return []domain.AuthoringStatus{domain.StatusApproved}, nil
	case domain.StatusApproved:
		return nil, nil
	default:
		return nil, domain.ErrInvalidStateTransition.WithInternal(
			fmt.Sprintf("cannot approve version in status %q", from),
		)
	}
}

// verifierName is the model recorded on the approval, or a placeholder when the
// caller did not name one.
func verifierName(model string) string {
	if trimmed := strings.TrimSpace(model); trimmed != "" {
		return trimmed
	}
	return "unknown"
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
	if err := s.attachTags(ctx, repo, item.ID, spec.Tags); err != nil {
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

// draftVersion writes one draft version and updates the item's current version pointer.
func (s *Service) draftVersion(
	ctx context.Context, repo Repository, itemID uuid.UUID, number int, spec contract.AuthorSpec,
) (uuid.UUID, error) {
	version, err := repo.CreateVersion(
		ctx, s.newID(), itemID, number, spec.Kind, spec.Body,
		spec.CEFRLevel, domain.StatusDraft, nil, nil,
	)
	if err != nil {
		return uuid.Nil, err
	}

	if _, err := repo.UpdateItemCurrentVersion(ctx, itemID, &version.ID); err != nil {
		return uuid.Nil, err
	}
	return version.ID, nil
}

// attachTags resolves tags against content.taxonomies and links them to the item.
func (s *Service) attachTags(ctx context.Context, repo Repository, itemID uuid.UUID, tags []contract.TagRef) error {
	for _, tag := range tags {
		tax, err := repo.GetTaxonomyByNamespaceCode(ctx, tag.Namespace, tag.Code)
		if err != nil {
			if errors.Is(err, domain.ErrTaxonomyNotFound) {
				return apperr.New(apperr.Validation, "CONTENT_TAG_NOT_FOUND",
					fmt.Sprintf("Taxonomy tag %s:%s not found.", tag.Namespace, tag.Code))
			}
			return fmt.Errorf("resolve tag %s:%s: %w", tag.Namespace, tag.Code, err)
		}
		if err := repo.AddContentTag(ctx, itemID, tax.ID); err != nil {
			return fmt.Errorf("add content tag %s:%s: %w", tag.Namespace, tag.Code, err)
		}
	}
	return nil
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
	return jsonEqual(version.Body, spec.Body)
}

func sameDraftBody(version domain.Version, spec contract.AuthorSpec) bool {
	if version.Kind != spec.Kind || version.CEFRLevel != spec.CEFRLevel {
		return false
	}
	if version.Status != domain.StatusDraft {
		return false
	}
	return jsonEqual(version.Body, spec.Body)
}

func jsonEqual(a, b json.RawMessage) bool {
	var stored, wanted any
	if err := json.Unmarshal(a, &stored); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &wanted); err != nil {
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
var _ contract.VerificationRecorder = (*Service)(nil)
var _ contract.VerifiedBatchPublisher = (*Service)(nil)
var _ contract.RecordedApprover = (*Service)(nil)
