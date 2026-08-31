package storage_test

import (
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

const (
	r2Endpoint = "https://f10ae015f0b691bd45b9c68e345c96bb.r2.cloudflarestorage.com"

	// The only region worth naming: it is what R2 deployments should use, and
	// what the other suites presign under.
	regionAuto = "auto"
)

// TestValidateRegion_RejectsWhatR2Rejects pins the exact deployment that failed:
// an AWS region name against an R2 endpoint. R2 answers 400 InvalidRegionName on
// the upload — cross-origin, so the page cannot read the reason — and it does so
// only after the signature and the CORS policy have both passed.
func TestValidateRegion_RejectsWhatR2Rejects(t *testing.T) {
	t.Parallel()

	err := storage.ValidateRegion(r2Endpoint, "ap-southeast-1")
	if err == nil {
		t.Fatal("ap-southeast-1 against R2 must not start")
	}
	// The message has to carry the fix, because whoever reads it is looking at a
	// crashed boot and not at this file.
	for _, want := range []string{"ap-southeast-1", regionAuto, "r2.cloudflarestorage.com"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

func TestValidateRegion_AcceptsEveryRegionR2Accepts(t *testing.T) {
	t.Parallel()

	for _, region := range []string{regionAuto, "wnam", "enam", "weur", "eeur", "apac", "oc"} {
		if err := storage.ValidateRegion(r2Endpoint, region); err != nil {
			t.Errorf("R2 accepts %q but this refused it: %v", region, err)
		}
	}
	// Case and stray whitespace are an operator's, not a fault.
	if err := storage.ValidateRegion(r2Endpoint, "  AUTO "); err != nil {
		t.Errorf("padded, upper-case auto was refused: %v", err)
	}
}

// The check is scoped to R2 on purpose. AWS takes real region names and MinIO
// takes anything, so validating either would refuse working deployments.
func TestValidateRegion_LeavesOtherStoresAlone(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"aws":             "https://s3.ap-southeast-1.amazonaws.com",
		"minio":           "localhost:9000",
		"minio in docker": "http://minio:9000",
	}
	for name, endpoint := range cases {
		if err := storage.ValidateRegion(endpoint, "ap-southeast-1"); err != nil {
			t.Errorf("%s: a non-R2 endpoint must not be constrained: %v", name, err)
		}
	}
}

// The endpoint is matched case-insensitively and with or without a scheme,
// because both spellings reach here from configuration.
func TestValidateRegion_RecognisesTheEndpointHowItIsWritten(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{
		"f10ae0.R2.CloudflareStorage.com",
		"https://f10ae0.r2.cloudflarestorage.com",
		"f10ae0.r2.cloudflarestorage.com",
	} {
		if err := storage.ValidateRegion(endpoint, "us-east-1"); err == nil {
			t.Errorf("%s was not recognised as R2", endpoint)
		}
	}
}
