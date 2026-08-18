package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/platform/job"
)

const (
	erasureCheckerLockID   int64 = 1_700_000_051
	erasureCheckerInterval       = 6 * time.Hour

	// AnonymisedEmailSuffix marks an address the deletion job has replaced. The
	// `user` record survives erasure by design — aggregate statistics reference
	// it — so this suffix, not the row's absence, is what says the address is
	// gone. Shared with deletion.go so the writer and the checker cannot drift.
	AnonymisedEmailSuffix = "@anonymised.invalid"

	// retainedProviders are excluded from the completeness check because they
	// are accountability records that must outlive the account they describe.
	// `audit` is append-only by grant, not by discipline; the reasoning for
	// admin_actions is in internal/modules/admin/DECISIONS.md. Note that
	// admin_actions is not an export provider at all, so it needs no entry
	// here — it is named in that decision record, not in this list.
	providerAudit = "audit"
	providerUser  = "user"
)

// ErasureChecker scheduled job verifies that every registered module purged personal data for deleted users.
type ErasureChecker struct {
	repo      ErasureRepository
	providers []NamedExportable
}

// ErasureRepository defines the persistence methods required by ErasureChecker.
type ErasureRepository interface {
	GetProcessingDeletions(ctx context.Context, limit int32) ([]domain.DeletionRequest, error)
	UpdateDeletionStatus(
		ctx context.Context,
		id uuid.UUID,
		status domain.DeletionStatus,
		startedAt, completedAt *time.Time,
		errorMessage *string,
	) error
}

// NewErasureChecker creates a new ErasureChecker instance.
func NewErasureChecker(repo ErasureRepository, providers []NamedExportable) *ErasureChecker {
	return &ErasureChecker{
		repo:      repo,
		providers: providers,
	}
}

// CronJob returns the scheduled job definition.
func (c *ErasureChecker) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "user.erasure_completeness_checker",
		LockID:   erasureCheckerLockID,
		Interval: erasureCheckerInterval,
		Task:     c.Run,
	}
}

// Run executes the completeness check on all processing deletion requests.
func (c *ErasureChecker) Run(ctx context.Context) error {
	const batchSize = 100
	processing, err := c.repo.GetProcessingDeletions(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("fetch processing deletions: %w", err)
	}

	now := time.Now().UTC()
	for _, req := range processing {
		complete, unpurgedModule, err := c.VerifyUserErased(ctx, req.UserID)
		if err != nil {
			slog.WarnContext(ctx, "erasure completeness check error",
				"user_id", req.UserID, "error", err)
			continue
		}

		if complete {
			if err := c.repo.UpdateDeletionStatus(ctx, req.ID, domain.DeletionStatusCompleted, nil, &now, nil); err != nil {
				slog.ErrorContext(ctx, "failed to mark deletion complete after erasure verification",
					"user_id", req.UserID, "error", err)
			} else {
				slog.InfoContext(ctx, "erasure completeness check passed and deletion completed",
					"user_id", req.UserID)
			}
		} else {
			slog.WarnContext(ctx, "erasure completeness check pending: module still holds data",
				"user_id", req.UserID, "unpurged_module", unpurgedModule)
		}
	}
	return nil
}

// VerifyUserErased reports whether every provider that is supposed to erase has
// done so for userID.
//
// It fails closed throughout. This is a compliance control: the cost of wrongly
// reporting "still holds data" is one more scheduled run, and the cost of
// wrongly reporting "erased" is an unnoticed breach. So an unreadable answer,
// an unreachable provider, or no providers at all all count as not-yet-erased,
// never as done.
func (c *ErasureChecker) VerifyUserErased(ctx context.Context, userID uuid.UUID) (bool, string, error) {
	if len(c.providers) == 0 {
		return false, "", errors.New(
			"erasure completeness check has no providers: refusing to certify erasure it did not verify")
	}

	userIDStr := userID.String()

	for _, item := range c.providers {
		if item.Name == providerAudit {
			continue
		}
		if item.Provider == nil {
			return false, item.Name, fmt.Errorf("provider %s is registered but nil", item.Name)
		}

		data, err := item.Provider.ExportUserData(ctx, userIDStr)
		if err != nil {
			return false, item.Name, fmt.Errorf("provider %s ExportUserData error: %w", item.Name, err)
		}

		holds, err := hasUnpurgedPersonalData(item.Name, data)
		if err != nil {
			return false, item.Name, err
		}
		if holds {
			return false, item.Name, nil
		}
	}
	return true, "", nil
}

// hasUnpurgedPersonalData reports whether one provider's export still describes
// a person.
//
// It deliberately does not encode each module's field names. Doing so was the
// first version of this function, and it read a missed type assertion as "no
// data" — so a module that changed the shape of its export would have been
// certified as erased without anybody touching this file. The contract on
// Exportable is the only thing relied on here: an empty map means the module
// holds nothing.
func hasUnpurgedPersonalData(moduleName string, data map[string]interface{}) (bool, error) {
	if len(data) == 0 {
		return false, nil
	}

	// `user` is the exception: the row survives erasure so aggregate statistics
	// stay truthful, and the address is overwritten rather than removed. The
	// marker on the address is what says the erasure happened.
	if moduleName == providerUser {
		raw, present := data["email"]
		if !present || raw == nil {
			return false, nil
		}
		email, ok := raw.(string)
		if !ok {
			return true, fmt.Errorf(
				"provider %s: email is %T, not a string — cannot tell whether it was anonymised",
				moduleName, raw)
		}
		return !strings.HasSuffix(email, AnonymisedEmailSuffix), nil
	}

	for key, value := range data {
		if value == nil {
			continue
		}
		length, ok := collectionLen(value)
		if !ok {
			return true, fmt.Errorf(
				"provider %s: key %q is %T, which this check cannot measure", moduleName, key, value)
		}
		if length > 0 {
			return true, nil
		}
	}
	return false, nil
}

// collectionLen returns the length of v when it is a slice, array or map, and
// false for any other kind. The false is what makes the caller fail closed: a
// scalar where a collection was expected is a shape this check does not
// understand, and guessing "empty" would certify an erasure on the strength of
// not understanding the answer.
func collectionLen(v any) (int, bool) {
	switch kind := reflect.ValueOf(v); kind.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return kind.Len(), true
	default:
		return 0, false
	}
}
