package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcauth "github.com/fluentra/fluentra/internal/generated/auth/sqlc"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// TrustDevice records or refreshes a trusted device, and returns the row.
//
// A second sign-in from a browser that is already trusted refreshes the existing
// row rather than adding another, and it deliberately does not move the absolute
// expiry — see the note on the query.
func (r *Repository) TrustDevice(
	ctx context.Context, id, userID uuid.UUID, deviceIDHash []byte, label *string,
	idleWindow time.Duration, absoluteExpiry, now time.Time,
) (domain.TrustedDevice, error) {
	row, err := r.queries.TrustDevice(ctx, sqlcauth.TrustDeviceParams{
		ID:                id,
		UserID:            userID,
		DeviceIDHash:      deviceIDHash,
		Label:             label,
		IdleWindow:        intervalOf(idleWindow),
		AbsoluteExpiresAt: absoluteExpiry,
		Now:               now,
	})
	if err != nil {
		return domain.TrustedDevice{}, fmt.Errorf("trust device: %w", err)
	}
	return toTrustedDevice(row), nil
}

// ListTrustedDevices returns the account's live devices, most recently active
// first. A device past its absolute expiry is not listed: it has stopped being
// trusted whether or not anything has swept it.
func (r *Repository) ListTrustedDevices(ctx context.Context, userID uuid.UUID, now time.Time) (
	[]domain.TrustedDevice, error,
) {
	rows, err := r.queries.ListTrustedDevices(ctx, sqlcauth.ListTrustedDevicesParams{UserID: userID, Now: now})
	if err != nil {
		return nil, fmt.Errorf("list trusted devices: %w", err)
	}
	devices := make([]domain.TrustedDevice, 0, len(rows))
	for _, row := range rows {
		devices = append(devices, toTrustedDevice(row))
	}
	return devices, nil
}

// GetOwnedTrustedDevice reads a device that belongs to userID.
//
// A device that exists but belongs to somebody else is reported as not found,
// identically to one that never existed, because the SQL puts both the id and
// the owner in the WHERE clause.
func (r *Repository) GetOwnedTrustedDevice(ctx context.Context, deviceID, userID uuid.UUID) (
	domain.TrustedDevice, bool, error,
) {
	row, err := r.queries.GetOwnedTrustedDevice(ctx, sqlcauth.GetOwnedTrustedDeviceParams{
		ID: deviceID, UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TrustedDevice{}, false, nil
		}
		return domain.TrustedDevice{}, false, fmt.Errorf("get owned trusted device: %w", err)
	}
	return toTrustedDevice(row), true, nil
}

// UntrustDevice ends the trust. It reports whether a row changed, so a caller
// can tell "untrusted it" from "it was already untrusted".
func (r *Repository) UntrustDevice(ctx context.Context, deviceID uuid.UUID, now time.Time) (bool, error) {
	affected, err := r.queries.UntrustDevice(ctx, sqlcauth.UntrustDeviceParams{ID: deviceID, Now: now})
	if err != nil {
		return false, fmt.Errorf("untrust device: %w", err)
	}
	return affected > 0, nil
}

// UntrustAllDevicesForUser is what a password change, reset or suspension calls
// (BR-AUTH-25). A device that stayed trusted through a reset would be a ninety-
// day window the attacker keeps.
func (r *Repository) UntrustAllDevicesForUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	affected, err := r.queries.UntrustAllDevicesForUser(ctx, sqlcauth.UntrustAllDevicesForUserParams{
		UserID: userID, Now: now,
	})
	if err != nil {
		return 0, fmt.Errorf("untrust all devices: %w", err)
	}
	return int(affected), nil
}

// TouchTrustedDevice moves the idle clock forward. It is what makes the window
// sliding for the device as well as for the token.
func (r *Repository) TouchTrustedDevice(ctx context.Context, deviceID uuid.UUID, now time.Time) error {
	if _, err := r.queries.TouchTrustedDevice(ctx, sqlcauth.TouchTrustedDeviceParams{
		ID: deviceID, Now: now,
	}); err != nil {
		return fmt.Errorf("touch trusted device: %w", err)
	}
	return nil
}

// RevokeSessionsForDevice signs out every session opened on one device.
func (r *Repository) RevokeSessionsForDevice(ctx context.Context, deviceID uuid.UUID, now time.Time) (int, error) {
	affected, err := r.queries.RevokeSessionsForDevice(ctx, sqlcauth.RevokeSessionsForDeviceParams{
		TrustedDeviceID: &deviceID, Now: now,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke sessions for device: %w", err)
	}
	return int(affected), nil
}

// RevokeRefreshTokensForDevice is the renewal half of the same untrust.
func (r *Repository) RevokeRefreshTokensForDevice(
	ctx context.Context, deviceID uuid.UUID, now time.Time,
) (int, error) {
	affected, err := r.queries.RevokeRefreshTokensForDevice(ctx, sqlcauth.RevokeRefreshTokensForDeviceParams{
		TrustedDeviceID: &deviceID, Now: now,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke refresh tokens for device: %w", err)
	}
	return int(affected), nil
}

// intervalOf converts a Go duration to the interval the column holds.
//
// Microseconds and not months or days: a duration is an exact span, while
// Postgres treats a month as a calendar unit whose length depends on when it
// starts. Writing "1 month" and reading back something that differs by three
// days across February is not a difference this schema wants to carry.
func intervalOf(window time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: window.Microseconds(), Valid: true}
}

func durationOf(interval pgtype.Interval) time.Duration {
	if !interval.Valid {
		return 0
	}
	const hoursPerDay = 24
	return time.Duration(interval.Microseconds)*time.Microsecond +
		time.Duration(interval.Days)*hoursPerDay*time.Hour +
		time.Duration(interval.Months)*30*hoursPerDay*time.Hour
}

func toTrustedDevice(row sqlcauth.CoreTrustedDevice) domain.TrustedDevice {
	return domain.TrustedDevice{
		ID:             row.ID,
		UserID:         row.UserID,
		DeviceIDHash:   row.DeviceIDHash,
		Label:          row.Label,
		IdleWindow:     durationOf(row.IdleWindow),
		AbsoluteExpiry: row.AbsoluteExpiresAt,
		TrustedAt:      row.TrustedAt,
		LastSeenAt:     row.LastSeenAt,
		RevokedAt:      row.RevokedAt,
	}
}
