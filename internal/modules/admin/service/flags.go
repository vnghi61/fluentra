package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlcadmin "github.com/fluentra/fluentra/internal/generated/admin/sqlc"
	admincontract "github.com/fluentra/fluentra/internal/modules/admin/contract"
	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

type cachedFlag struct {
	flag      sqlcadmin.CoreFeatureFlag
	expiresAt time.Time
}

// FlagCache provides in-memory caching with 30s TTL for fast flag evaluation (< 5ms).
type FlagCache struct {
	mu    sync.RWMutex
	items map[string]cachedFlag
}

func newFlagCache() *FlagCache {
	return &FlagCache{
		items: make(map[string]cachedFlag),
	}
}

func (c *FlagCache) get(key string, now time.Time) (sqlcadmin.CoreFeatureFlag, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok || now.After(item.expiresAt) {
		return sqlcadmin.CoreFeatureFlag{}, false
	}
	return item.flag, true
}

func (c *FlagCache) set(key string, flag sqlcadmin.CoreFeatureFlag, ttl time.Duration, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cachedFlag{
		flag:      flag,
		expiresAt: now.Add(ttl),
	}
}

func (c *FlagCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

var localFlagCache = newFlagCache()

// IsEnabled evaluates whether a feature flag is enabled for the given user.
// Implements admincontract.FlagReader.
func (s *Service) IsEnabled(ctx context.Context, key string, userID uuid.UUID) (bool, error) {
	now := s.clock.Now()

	// 1. Check local cache (30s TTL)
	flag, found := localFlagCache.get(key, now)
	if !found {
		// 2. Fetch from repository DB
		var err error
		flag, err = s.repo.GetFeatureFlagByKey(ctx, key)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		localFlagCache.set(key, flag, 30*time.Second, now)
	}

	if !flag.Enabled {
		return false, nil
	}

	if flag.RolloutPercent >= 100 {
		return true, nil
	}
	if flag.RolloutPercent <= 0 {
		return false, nil
	}

	// Stable per-user bucketing via SHA256(key + userID) % 100
	bucket := computeBucket(key, userID)
	return bucket < int(flag.RolloutPercent), nil
}

func computeBucket(key string, userID uuid.UUID) int {
	h := sha256.New()
	h.Write([]byte(key))
	h.Write(userID[:])
	sum := h.Sum(nil)
	return int(binary.BigEndian.Uint32(sum[:4]) % 100)
}

// ListFlags retrieves all feature flags.
func (s *Service) ListFlags(ctx context.Context) ([]admincontract.FeatureFlag, error) {
	rows, err := s.repo.ListFeatureFlags(ctx)
	if err != nil {
		return nil, err
	}

	flags := make([]admincontract.FeatureFlag, 0, len(rows))
	for _, r := range rows {
		flags = append(flags, toContractFlag(r))
	}
	return flags, nil
}

// CreateFlagRequest holds input for creating a flag.
type CreateFlagRequest struct {
	Key            string    `json:"key"`
	Enabled        bool      `json:"enabled"`
	RolloutPercent int       `json:"rollout_percent"`
	Owner          string    `json:"owner"`
	ExpiresOn      time.Time `json:"expires_on"`
	Description    string    `json:"description"`
}

// CreateFlag validates and creates a new feature flag.
func (s *Service) CreateFlag(ctx context.Context, req CreateFlagRequest) (admincontract.FeatureFlag, error) {
	if req.Key == "" {
		return admincontract.FeatureFlag{}, apperr.New(apperr.Validation, "KEY_REQUIRED", "Flag key is required.")
	}
	if req.Owner == "" {
		return admincontract.FeatureFlag{}, apperr.New(apperr.Validation, "OWNER_REQUIRED", "Flag owner is required.")
	}
	if req.Description == "" {
		return admincontract.FeatureFlag{}, apperr.New(
			apperr.Validation, "DESCRIPTION_REQUIRED", "Flag description is required.")
	}
	if req.ExpiresOn.Before(s.clock.Now()) {
		return admincontract.FeatureFlag{}, apperr.New(
			apperr.Validation, "INVALID_EXPIRY", "Expiry date must be in the future.")
	}
	if req.RolloutPercent < 0 || req.RolloutPercent > 100 {
		return admincontract.FeatureFlag{}, apperr.New(
			apperr.Validation, "INVALID_ROLLOUT", "Rollout percent must be between 0 and 100.")
	}

	row, err := s.repo.CreateFeatureFlag(
		ctx,
		req.Key,
		req.Enabled,
		int32(req.RolloutPercent),
		req.Owner,
		req.ExpiresOn,
		req.Description,
	)
	if err != nil {
		return admincontract.FeatureFlag{}, err
	}

	localFlagCache.invalidate(req.Key)
	return toContractFlag(row), nil
}

// UpdateFlagRequest holds input for updating a flag.
type UpdateFlagRequest struct {
	Enabled        bool      `json:"enabled"`
	RolloutPercent int       `json:"rollout_percent"`
	ExpiresOn      time.Time `json:"expires_on"`
	Description    string    `json:"description"`
}

// UpdateFlag modifies an existing feature flag.
func (s *Service) UpdateFlag(
	ctx context.Context, key string, req UpdateFlagRequest,
) (admincontract.FeatureFlag, error) {
	if req.RolloutPercent < 0 || req.RolloutPercent > 100 {
		return admincontract.FeatureFlag{}, apperr.New(
			apperr.Validation, "INVALID_ROLLOUT", "Rollout percent must be between 0 and 100.")
	}
	if req.ExpiresOn.Before(s.clock.Now()) {
		return admincontract.FeatureFlag{}, apperr.New(
			apperr.Validation, "INVALID_EXPIRY", "Expiry date must be in the future.")
	}

	row, err := s.repo.UpdateFeatureFlag(
		ctx,
		key,
		req.Enabled,
		int32(req.RolloutPercent),
		req.ExpiresOn,
		req.Description,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return admincontract.FeatureFlag{}, admindomain.ErrFlagNotFound
		}
		return admincontract.FeatureFlag{}, err
	}

	localFlagCache.invalidate(key)
	return toContractFlag(row), nil
}

// DeleteFlag deletes a feature flag.
func (s *Service) DeleteFlag(ctx context.Context, key string) error {
	if err := s.repo.DeleteFeatureFlag(ctx, key); err != nil {
		return err
	}
	localFlagCache.invalidate(key)
	return nil
}

func toContractFlag(row sqlcadmin.CoreFeatureFlag) admincontract.FeatureFlag {
	var expiresStr string
	if row.ExpiresOn.Valid {
		expiresStr = row.ExpiresOn.Time.Format("2006-01-02")
	}
	return admincontract.FeatureFlag{
		Key:            row.Key,
		Enabled:        row.Enabled,
		RolloutPercent: int(row.RolloutPercent),
		Owner:          row.Owner,
		ExpiresOn:      expiresStr,
		Description:    row.Description,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      row.UpdatedAt.Format(time.RFC3339),
	}
}

// ReportExpiringFlags logs feature flags expiring within 7 days.
func (s *Service) ReportExpiringFlags(ctx context.Context) error {
	cutoff := s.clock.Now().Add(7 * 24 * time.Hour)
	flags, err := s.repo.GetFlagsExpiringWithin(ctx, cutoff)
	if err != nil {
		return err
	}

	for _, flag := range flags {
		var expiresStr string
		if flag.ExpiresOn.Valid {
			expiresStr = flag.ExpiresOn.Time.Format("2006-01-02")
		}
		slog.InfoContext(ctx, "feature flag expiring soon",
			"key", flag.Key,
			"owner", flag.Owner,
			"expires_on", expiresStr,
		)
	}
	return nil
}
