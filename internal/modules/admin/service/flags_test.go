package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcadmin "github.com/fluentra/fluentra/internal/generated/admin/sqlc"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
)

type fakeFlagRepo struct {
	flags map[string]sqlcadmin.CoreFeatureFlag
}

func (f *fakeFlagRepo) LogAdminAction(
	_ context.Context, _, _ uuid.UUID, _, _ string,
) (sqlcadmin.CoreAdminAction, error) {
	return sqlcadmin.CoreAdminAction{}, nil
}

func (f *fakeFlagRepo) ListFeatureFlags(_ context.Context) ([]sqlcadmin.CoreFeatureFlag, error) {
	list := make([]sqlcadmin.CoreFeatureFlag, 0, len(f.flags))
	for _, fl := range f.flags {
		list = append(list, fl)
	}
	return list, nil
}

func (f *fakeFlagRepo) GetFeatureFlagByKey(_ context.Context, key string) (sqlcadmin.CoreFeatureFlag, error) {
	fl, ok := f.flags[key]
	if !ok {
		return sqlcadmin.CoreFeatureFlag{}, nil
	}
	return fl, nil
}

func (f *fakeFlagRepo) CreateFeatureFlag(
	_ context.Context, key string, enabled bool, rollout int32, owner string,
	exp time.Time, desc string,
) (sqlcadmin.CoreFeatureFlag, error) {
	fl := sqlcadmin.CoreFeatureFlag{
		Key:            key,
		Enabled:        enabled,
		RolloutPercent: rollout,
		Owner:          owner,
		ExpiresOn:      pgtype.Date{Time: exp, Valid: true},
		Description:    desc,
	}
	f.flags[key] = fl
	return fl, nil
}

func (f *fakeFlagRepo) UpdateFeatureFlag(
	_ context.Context, key string, enabled bool, rollout int32, exp time.Time, desc string,
) (sqlcadmin.CoreFeatureFlag, error) {
	fl := f.flags[key]
	fl.Enabled = enabled
	fl.RolloutPercent = rollout
	fl.ExpiresOn = pgtype.Date{Time: exp, Valid: true}
	fl.Description = desc
	f.flags[key] = fl
	return fl, nil
}

func (f *fakeFlagRepo) DeleteFeatureFlag(_ context.Context, key string) error {
	delete(f.flags, key)
	return nil
}

func (f *fakeFlagRepo) GetFlagsExpiringWithin(_ context.Context, _ time.Time) ([]sqlcadmin.CoreFeatureFlag, error) {
	return nil, nil
}

func TestFlagBucketingStability(t *testing.T) {
	repo := &fakeFlagRepo{flags: map[string]sqlcadmin.CoreFeatureFlag{
		"new_checkout": {
			Key:            "new_checkout",
			Enabled:        true,
			RolloutPercent: 50,
			Owner:          "payments-team",
		},
	}}

	svc := adminsvc.New(adminsvc.Deps{Repo: repo})
	userID := uuid.New()

	firstEval, err := svc.IsEnabled(context.Background(), "new_checkout", userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 10; i++ {
		eval, err := svc.IsEnabled(context.Background(), "new_checkout", userID)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if eval != firstEval {
			t.Fatalf("bucketing not stable: got %v, expected %v", eval, firstEval)
		}
	}
}

func TestFlagRolloutPercentageDistribution(t *testing.T) {
	repo := &fakeFlagRepo{flags: map[string]sqlcadmin.CoreFeatureFlag{
		"test_feature": {
			Key:            "test_feature",
			Enabled:        true,
			RolloutPercent: 50,
			Owner:          "core-team",
		},
	}}

	svc := adminsvc.New(adminsvc.Deps{Repo: repo})
	enabledCount := 0
	totalUsers := 200

	for i := 0; i < totalUsers; i++ {
		userID := uuid.New()
		enabled, err := svc.IsEnabled(context.Background(), "test_feature", userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enabled {
			enabledCount++
		}
	}

	// For 50% rollout across 200 users, enabledCount should be roughly between 70 and 130
	if enabledCount < 70 || enabledCount > 130 {
		t.Fatalf("unexpected rollout distribution: %d out of %d enabled (expected ~100)", enabledCount, totalUsers)
	}
}
