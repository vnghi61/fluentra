package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// DeviceView is one row of the caller's trusted-device list.
//
// Both expiries are here, and showing both is the point: "stay signed in" is
// only defensible because it ends, and a learner who can see the fixed date can
// reason about the risk instead of taking it on faith.
type DeviceView struct {
	ID                uuid.UUID
	Current           bool
	Label             *string
	TrustedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// DeviceRepo is the persistence the device service needs.
type DeviceRepo interface {
	ListTrustedDevices(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.TrustedDevice, error)
	GetOwnedTrustedDevice(ctx context.Context, deviceID, userID uuid.UUID) (domain.TrustedDevice, bool, error)
	// GetOwnedSession is how the service learns which device the caller is on:
	// the current session carries the trusted-device id it was opened for, and
	// that id is what "current" in the list is compared against.
	GetOwnedSession(ctx context.Context, sessionID, userID uuid.UUID) (domain.Session, bool, error)
	UntrustDevice(ctx context.Context, deviceID uuid.UUID, now time.Time) (bool, error)
	RevokeSessionsForDevice(ctx context.Context, deviceID uuid.UUID, now time.Time) (int, error)
	RevokeRefreshTokensForDevice(ctx context.Context, deviceID uuid.UUID, now time.Time) (int, error)
	WithTx(tx pgx.Tx) DeviceRepo
}

// DeviceDeps are the service's collaborators.
type DeviceDeps struct {
	Pool  dbx.Beginner
	Repo  DeviceRepo
	Clock clock.Clock
}

// DeviceService lists and untrusts devices.
type DeviceService struct {
	pool  dbx.Beginner
	repo  DeviceRepo
	clock clock.Clock
}

// NewDeviceService creates the service.
func NewDeviceService(deps DeviceDeps) *DeviceService {
	return &DeviceService{pool: deps.Pool, repo: deps.Repo, clock: deps.Clock}
}

// List returns the caller's own trusted devices.
//
// The account comes from the token, so there is no version of this that reads
// somebody else's list and no path segment for a caller to change.
func (s *DeviceService) List(ctx context.Context, actor httpx.Actor) ([]DeviceView, error) {
	devices, err := s.repo.ListTrustedDevices(ctx, actor.UserID, s.clock.Now().UTC())
	if err != nil {
		return nil, err
	}

	// The current device is the one the caller's own session was opened for. If
	// the session cannot be read — it was just revoked, say — no device is
	// marked current, which is the safe direction: the interface then warns
	// "if this is the device you are on" rather than asserting it confidently.
	var (
		currentID  uuid.UUID
		hasCurrent bool
	)
	if session, found, err := s.repo.GetOwnedSession(ctx, actor.SessionID, actor.UserID); err == nil &&
		found && session.TrustedDeviceID != nil {
		currentID = *session.TrustedDeviceID
		hasCurrent = true
	}

	views := make([]DeviceView, 0, len(devices))
	for _, device := range devices {
		views = append(views, DeviceView{
			ID:                device.ID,
			Current:           hasCurrent && device.ID == currentID,
			Label:             device.Label,
			TrustedAt:         device.TrustedAt,
			LastSeenAt:        device.LastSeenAt,
			IdleExpiresAt:     device.IdleExpiresAt(),
			AbsoluteExpiresAt: device.AbsoluteExpiry,
		})
	}
	return views, nil
}

// Untrust stops trusting a device and signs it out.
//
// **The refresh family goes immediately**, not at the end of a shorter window.
// Untrusting is what a learner reaches for when a laptop is lost, and a laptop
// demoted to thirty days is still a laptop somebody else is signed in on.
//
// A device belonging to another account is `RESOURCE_NOT_FOUND`, the same answer
// an id that never existed gets, and for the same reason `DELETE
// /auth/sessions/{id}` gives: 403 would confirm the id names a real device.
//
// Untrusting an already-untrusted device succeeds. The caller asked for a state
// and the state holds.
func (s *DeviceService) Untrust(ctx context.Context, actor httpx.Actor, deviceID uuid.UUID) error {
	_, found, err := s.repo.GetOwnedTrustedDevice(ctx, deviceID, actor.UserID)
	if err != nil {
		return err
	}
	if !found {
		return errDeviceNotFound()
	}

	now := s.clock.Now().UTC()
	return s.inTx(ctx, func(ctx context.Context, repo DeviceRepo) error {
		// The renewals go before the trust does, for the reason session
		// revocation orders the same way: they commit together, so neither
		// half-state is reachable, but the unreachable one is then the harmless
		// direction — a device still marked trusted that cannot renew expires
		// quietly, while the reverse renews itself.
		if _, err := repo.RevokeRefreshTokensForDevice(ctx, deviceID, now); err != nil {
			return err
		}
		if _, err := repo.RevokeSessionsForDevice(ctx, deviceID, now); err != nil {
			return err
		}
		_, err := repo.UntrustDevice(ctx, deviceID, now)
		return err
	})
}

func (s *DeviceService) inTx(ctx context.Context, fn func(context.Context, DeviceRepo) error) error {
	// READ COMMITTED, for the reason set out on RefreshService.inTx: every write
	// here is an UPDATE guarded on `revoked_at IS NULL`, so correctness is a row
	// predicate rather than a snapshot.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(ctx, s.repo.WithTx(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func errDeviceNotFound() error {
	return apperr.New(apperr.NotFound, "RESOURCE_NOT_FOUND", "That device was not found.")
}
