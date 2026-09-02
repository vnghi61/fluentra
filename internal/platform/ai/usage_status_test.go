package ai_test

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/ai"
)

// The constants and the CHECK constraint have to agree.
//
// `ai.ai_requests` refuses any status outside its four permitted values, and it
// refuses it at insert time, inside whatever background job was recording the
// request. The failure is a lost usage row rather than anything a learner sees,
// which is the reason to catch it here: nobody is watching that path closely
// enough to notice it going quiet.
//
// Adding a fifth status to the migration without adding the constant leaves a
// value no Go code can produce, which is harmless. Adding one to Go without the
// migration is the direction that breaks, so the test is written to fail on it.
func TestRequestStatus_MatchesTheCheckConstraint(t *testing.T) {
	const migrationPath = "../../../db/migrations/ai/1700000310_create_ai_tables.sql"

	body, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read ai migration: %v", err)
	}

	constraint := regexp.MustCompile(`ck_ai_requests_status CHECK \(status IN \(([^)]*)\)\)`).
		FindStringSubmatch(string(body))
	if constraint == nil {
		t.Fatal("ck_ai_requests_status is not in the migration; either it was renamed or the " +
			"status column no longer constrains what can be written")
	}

	permitted := map[string]bool{}
	for _, raw := range strings.Split(constraint[1], ",") {
		permitted[strings.Trim(strings.TrimSpace(raw), "'")] = true
	}

	declared := []ai.RequestStatus{
		ai.StatusSuccess, ai.StatusFailed, ai.StatusCached, ai.StatusRateLimited,
	}
	for _, status := range declared {
		if !permitted[string(status)] {
			t.Errorf("ai.RequestStatus %q is not accepted by ck_ai_requests_status; recording it "+
				"would fail the insert", status)
		}
	}

	if len(permitted) != len(declared) {
		missing := make([]string, 0, len(permitted))
		for value := range permitted {
			missing = append(missing, value)
		}
		sort.Strings(missing)
		t.Errorf("the constraint permits %d statuses %v but only %d constants exist; a status no "+
			"constant names cannot be written by any caller", len(permitted), missing, len(declared))
	}
}
