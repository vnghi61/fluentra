package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	userdomain "github.com/fluentra/fluentra/internal/modules/user/domain"
	userjob "github.com/fluentra/fluentra/internal/modules/user/job"
)

const (
	modNameUser      = "user"
	modNameAuth      = "auth"
	modNameRBAC      = "rbac"
	modNameAudit     = "audit"
	keyFieldEmail    = "email"
	keyFieldSessions = "sessions"
	keyFieldRoles    = "roles"
)

type fakeErasureRepo struct {
	processing []userdomain.DeletionRequest
	completed  map[uuid.UUID]bool
}

func (f *fakeErasureRepo) GetProcessingDeletions(_ context.Context, _ int32) ([]userdomain.DeletionRequest, error) {
	return f.processing, nil
}

func (f *fakeErasureRepo) UpdateDeletionStatus(
	_ context.Context,
	id uuid.UUID,
	status userdomain.DeletionStatus,
	_, _ *time.Time,
	_ *string,
) error {
	if status == userdomain.DeletionStatusCompleted {
		f.completed[id] = true
	}
	return nil
}

type fakeProvider struct {
	name string
	data map[string]interface{}
}

func (f *fakeProvider) ExportUserData(_ context.Context, _ string) (map[string]interface{}, error) {
	return f.data, nil
}

func TestErasureChecker_MarksCompletedWhenAllPurged(t *testing.T) {
	targetID := uuid.New()
	reqID := uuid.New()

	repo := &fakeErasureRepo{
		processing: []userdomain.DeletionRequest{
			{ID: reqID, UserID: targetID, Status: userdomain.DeletionStatusProcessing},
		},
		completed: make(map[uuid.UUID]bool),
	}

	anonEmail := "deleted-" + targetID.String() + "@anonymised.invalid"
	providers := []userjob.NamedExportable{
		{
			Name: modNameUser,
			Provider: &fakeProvider{
				name: modNameUser,
				data: map[string]interface{}{keyFieldEmail: anonEmail},
			},
		},
		{
			Name: modNameAuth,
			Provider: &fakeProvider{
				name: modNameAuth,
				data: map[string]interface{}{keyFieldSessions: []map[string]interface{}{}},
			},
		},
		{
			Name: modNameRBAC,
			Provider: &fakeProvider{
				name: modNameRBAC,
				data: map[string]interface{}{keyFieldRoles: []string{}},
			},
		},
		{
			Name: modNameAudit,
			Provider: &fakeProvider{
				name: modNameAudit,
				data: map[string]interface{}{"logs": []string{"log1"}},
			},
		},
	}

	checker := userjob.NewErasureChecker(repo, providers)
	if err := checker.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error running ErasureChecker: %v", err)
	}

	if !repo.completed[reqID] {
		t.Fatalf("expected deletion request %s to be marked completed", reqID)
	}
}

func TestErasureChecker_LeavesProcessingWhenUnpurged(t *testing.T) {
	targetID := uuid.New()
	reqID := uuid.New()

	repo := &fakeErasureRepo{
		processing: []userdomain.DeletionRequest{
			{ID: reqID, UserID: targetID, Status: userdomain.DeletionStatusProcessing},
		},
		completed: make(map[uuid.UUID]bool),
	}

	anonEmail := "deleted-" + targetID.String() + "@anonymised.invalid"
	providers := []userjob.NamedExportable{
		{
			Name: modNameUser,
			Provider: &fakeProvider{
				name: modNameUser,
				data: map[string]interface{}{keyFieldEmail: anonEmail},
			},
		},
		{
			Name: modNameAuth,
			Provider: &fakeProvider{
				name: modNameAuth,
				data: map[string]interface{}{
					keyFieldSessions: []map[string]interface{}{{"id": "sess-1"}},
				},
			},
		},
		{
			Name: modNameRBAC,
			Provider: &fakeProvider{
				name: modNameRBAC,
				data: map[string]interface{}{keyFieldRoles: []string{}},
			},
		},
	}

	checker := userjob.NewErasureChecker(repo, providers)
	if err := checker.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error running ErasureChecker: %v", err)
	}

	if repo.completed[reqID] {
		t.Fatalf("expected deletion request %s to stay processing due to unpurged auth sessions", reqID)
	}
}

// The three tests below cover the ways this check can be asked a question it
// cannot answer. All of them must resolve to "not erased", because the failure
// they guard against is certifying an erasure that never happened.

func TestErasureChecker_RefusesToCertifyWithNoProviders(t *testing.T) {
	checker := userjob.NewErasureChecker(&fakeErasureRepo{completed: map[uuid.UUID]bool{}}, nil)

	erased, _, err := checker.VerifyUserErased(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected an error when no providers are registered, got nil")
	}
	if erased {
		t.Fatal("certified erasure without a single provider to verify it against")
	}
}

// A provider that changes the shape of its export must not be read as having
// erased. This is the regression the field-name-aware first version allowed:
// a missed type assertion looked exactly like an empty collection.
func TestErasureChecker_RefusesToCertifyAnUnreadableShape(t *testing.T) {
	providers := []userjob.NamedExportable{
		{Name: modNameAuth, Provider: &fakeProvider{
			name: modNameAuth,
			// A count where the checker expects a collection: truthful data,
			// but not a shape it can measure.
			data: map[string]interface{}{keyFieldSessions: 3},
		}},
	}
	checker := userjob.NewErasureChecker(&fakeErasureRepo{completed: map[uuid.UUID]bool{}}, providers)

	erased, module, err := checker.VerifyUserErased(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected an error for an unmeasurable export shape, got nil")
	}
	if erased {
		t.Fatal("certified erasure from an export shape it could not read")
	}
	if module != modNameAuth {
		t.Fatalf("blamed module %q, want %q", module, modNameAuth)
	}
}

func TestErasureChecker_ReadsTheAnonymisedAddressMarker(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		email      any
		wantErased bool
		wantErr    bool
	}{
		{name: "anonymised", email: "deleted-" + uuid.New().String() + userjob.AnonymisedEmailSuffix, wantErased: true},
		{name: "still the learner's", email: "learner@example.com", wantErased: false},
		{name: "unreadable type", email: 42, wantErased: false, wantErr: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			providers := []userjob.NamedExportable{
				{Name: modNameUser, Provider: &fakeProvider{
					name: modNameUser,
					data: map[string]interface{}{keyFieldEmail: testCase.email},
				}},
			}
			checker := userjob.NewErasureChecker(&fakeErasureRepo{completed: map[uuid.UUID]bool{}}, providers)

			erased, _, err := checker.VerifyUserErased(context.Background(), uuid.New())
			if testCase.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if erased != testCase.wantErased {
				t.Fatalf("erased = %v, want %v", erased, testCase.wantErased)
			}
		})
	}
}
