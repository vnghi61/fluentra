package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/service"
)

// TestApproveVerified_RefusesAnUnconfirmedVerification. The door publishes; it
// never judges. A verification that doubted anything is refused and stays for a
// person (WO 22 Stage A).
func TestApproveVerified_RefusesAnUnconfirmedVerification(t *testing.T) {
	svc := service.New(service.Deps{})

	err := svc.ApproveVerified(context.Background(), uuid.New(), contract.Verification{
		Confirmed: false,
		Reason:    "second option defensible",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CONTENT_VERIFICATION_NOT_CONFIRMED")
}

func TestApproveVerified_RequiresAVersion(t *testing.T) {
	svc := service.New(service.Deps{})

	err := svc.ApproveVerified(context.Background(), uuid.Nil, contract.Verification{Confirmed: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CONTENT_VERSION_REQUIRED")
}
