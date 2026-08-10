package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// The service is what other modules get when they ask for the user contract.
// Declaring the assertions here means a change to either side is a compile
// error in this package rather than a runtime surprise in whichever module
// depends on it.
var (
	_ contract.Reader    = (*Service)(nil)
	_ contract.Creator   = (*Service)(nil)
	_ contract.Registrar = (*Service)(nil)
)

// GetByID returns one rendering summary.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (contract.Summary, error) {
	summary, err := s.repo.GetSummary(ctx, id)
	if err != nil {
		return contract.Summary{}, err
	}
	return toContractSummary(summary), nil
}

// GetManyByIDs returns the summaries that exist among ids.
//
// One repository call, one SQL statement, for any number of ids. The map is
// keyed rather than ordered because the caller already has the ids and wants
// to look them up; returning a slice would push a second loop onto every
// caller, which is how the N+1 this method exists to prevent gets rebuilt in
// the rendering layer.
func (s *Service) GetManyByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]contract.Summary, error) {
	unique := deduplicate(ids)
	if len(unique) == 0 {
		return map[uuid.UUID]contract.Summary{}, nil
	}

	summaries, err := s.repo.ListSummaries(ctx, unique)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]contract.Summary, len(summaries))
	for _, summary := range summaries {
		result[summary.ID] = toContractSummary(summary)
	}
	return result, nil
}

// Exists reports whether the account exists.
func (s *Service) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	return s.repo.Exists(ctx, id)
}

// CreateUser writes the identity, profile and preference rows in one
// transaction. `auth` calls this during registration.
//
// All three rows are written together because every reader in the system
// assumes they exist: a user with no preference row would make
// GET /me/preferences a 404 for an account that just registered successfully.
func (s *Service) CreateUser(ctx context.Context, newUser contract.NewUser) (uuid.UUID, error) {
	if err := domain.ValidateDisplayName(newUser.DisplayName); err != nil {
		return uuid.Nil, err
	}
	timezone := newUser.Timezone
	if timezone == "" {
		timezone = domain.DefaultTimezone
	}
	if err := domain.ValidateTimezone(timezone); err != nil {
		return uuid.Nil, err
	}

	userID, err := s.newID(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	profileID, err := s.newID(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	preferencesID, err := s.newID(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	err = dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)
		if _, createErr := repo.CreateUser(ctx, userID, newUser.Email, domain.StatusActive); createErr != nil {
			return createErr
		}
		profile := domain.Profile{DisplayName: newUser.DisplayName, Timezone: timezone}
		if _, createErr := repo.CreateProfile(ctx, profileID, userID, profile); createErr != nil {
			return createErr
		}
		_, createErr := repo.CreatePreferences(ctx, preferencesID, userID)
		return createErr
	})
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// FindByEmail reports whether an address is registered, and whether it has been
// verified. An unknown address is `false, nil` rather than an error: the caller
// is `auth` deciding between two registration paths, and both are ordinary.
func (s *Service) FindByEmail(ctx context.Context, email string) (contract.Account, bool, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if apperr.Is(err, apperr.NotFound) {
			return contract.Account{}, false, nil
		}
		return contract.Account{}, false, err
	}
	return contract.Account{
		ID:       user.ID,
		Verified: user.EmailVerified(),
		Status:   string(user.Status),
	}, true, nil
}

// Recipient returns the account's mailing details.
func (s *Service) Recipient(ctx context.Context, userID uuid.UUID) (contract.Contact, error) {
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return contract.Contact{}, err
	}
	summary, err := s.repo.GetSummary(ctx, userID)
	if err != nil {
		return contract.Contact{}, err
	}
	return contract.Contact{
		Email:       user.Email,
		DisplayName: summary.DisplayName,
		Locale:      summary.Locale,
	}, nil
}

// MarkEmailVerified records that the address was proved.
func (s *Service) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := s.repo.MarkEmailVerified(ctx, userID)
	return err
}

// PurgeUnverifiedBefore deletes accounts that never completed verification.
func (s *Service) PurgeUnverifiedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	return s.repo.PurgeUnverifiedBefore(ctx, cutoff)
}

// toContractSummary converts the internal shape to the published one. The
// avatar is still nil: turning an asset id into a URL needs the storage facade,
// which arrives with P3.1.
func toContractSummary(summary domain.Summary) contract.Summary {
	return contract.Summary{
		ID:          summary.ID,
		DisplayName: summary.DisplayName,
		AvatarURL:   nil,
		Locale:      summary.Locale,
		Timezone:    summary.Timezone,
		Status:      string(summary.Status),
	}
}

// deduplicate keeps the caller honest about the query it asked for: a list
// with the same id three times is one row, and passing the duplicates through
// to `= ANY($1)` would only make the result harder to reason about.
func deduplicate(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
