package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

const validPreferencesBody = `{
	"locale": "vi",
	"theme": "dark",
	"daily_goal_minutes": 30,
	"notification_channels": ["in_app", "push"],
	"quiet_hours": {"start": "22:00", "end": "07:00"},
	"ai_processing_opt_out": false
}`

func TestGetPreferences_ReadsTheActorsOwnSettings(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/me/preferences", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	if accounts.seenActor != actorID {
		t.Errorf("service was asked for %s, want %s", accounts.seenActor, actorID)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{
		"locale", "theme", "daily_goal_minutes", "notification_channels",
		"quiet_hours", "ai_processing_opt_out", "updated_at",
	} {
		if _, present := body[field]; !present {
			t.Errorf("response is missing %q", field)
		}
	}

	window, ok := body["quiet_hours"].(map[string]any)
	if !ok {
		t.Fatalf("quiet_hours = %T, want an object", body["quiet_hours"])
	}
	if window["start"] != "22:00" || window["end"] != "07:00" {
		t.Errorf("quiet hours = %v, want 22:00 to 07:00 in HH:MM form", window)
	}
}

func TestPutPreferences_StoresEveryFieldSubmitted(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/preferences", validPreferencesBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	wanted := accounts.seenWanted
	if wanted.Locale != "vi" || wanted.Theme != domain.ThemeDark || wanted.DailyGoalMinutes != 30 {
		t.Errorf("wanted = %+v, want the submitted values", wanted)
	}
	if wanted.AIProcessingOptOut {
		t.Error("ai_processing_opt_out was submitted false and arrived true")
	}
	if wanted.QuietHours == nil || wanted.QuietHours.Start.Hour != 22 || wanted.QuietHours.End.Hour != 7 {
		t.Errorf("quiet hours = %+v, want 22:00 to 07:00", wanted.QuietHours)
	}
	if len(wanted.NotificationChannels) != 2 {
		t.Errorf("channels = %v, want two", wanted.NotificationChannels)
	}
}

// TestPutPreferences_MissingFieldIsRejected is what makes PUT idempotent. If a
// missing field meant "leave it alone", replaying the same request against a
// changed resource would produce a different result.
func TestPutPreferences_MissingFieldIsRejected(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	body := `{"locale":"vi","theme":"dark","daily_goal_minutes":30}`
	recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/preferences", body)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}

	decoded := decodeProblem(t, recorder)
	missing := map[string]bool{}
	for _, violation := range decoded.Errors {
		if violation.Code != "REQUIRED" {
			t.Errorf("violation = %+v, want REQUIRED", violation)
		}
		missing[violation.Field] = true
	}
	for _, field := range []string{"notification_channels", "ai_processing_opt_out"} {
		if !missing[field] {
			t.Errorf("%q was not reported as required", field)
		}
	}
	if accounts.seenWanted.Locale != "" {
		t.Error("the service was called despite the rejected body")
	}
}

// TestPutPreferences_QuietHoursIsOptional is the one nullable member: absent
// and null both mean "no quiet hours", because for a replacement there is no
// "leave it alone" for them to be confused with.
func TestPutPreferences_QuietHoursIsOptional(t *testing.T) {
	t.Parallel()

	bodies := map[string]string{
		"omitted": `{"locale":"vi","theme":"dark","daily_goal_minutes":30,` +
			`"notification_channels":["in_app"],"ai_processing_opt_out":false}`,
		"null": `{"locale":"vi","theme":"dark","daily_goal_minutes":30,` +
			`"notification_channels":["in_app"],"quiet_hours":null,"ai_processing_opt_out":false}`,
	}
	for label, body := range bodies {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			accounts := &fakeAccounts{}
			server := newServer(accounts)

			recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/preferences", body)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
			}
			if accounts.seenWanted.QuietHours != nil {
				t.Errorf("quiet hours = %+v, want nil", accounts.seenWanted.QuietHours)
			}
		})
	}
}

func TestPutPreferences_RejectsAMalformedQuietHoursWindow(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	cases := map[string]string{
		"single digit hour": `{"start":"9:00","end":"17:00"}`,
		"seconds included":  `{"start":"22:00:00","end":"07:00"}`,
		"hour out of range": `{"start":"24:00","end":"07:00"}`,
		"not a time":        `{"start":"evening","end":"morning"}`,
	}
	for label, window := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			body := `{"locale":"vi","theme":"dark","daily_goal_minutes":30,` +
				`"notification_channels":["in_app"],"quiet_hours":` + window +
				`,"ai_processing_opt_out":false}`
			recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/preferences", body)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
			}
			if got := decodeProblem(t, recorder).Errors[0].Field; got != "quiet_hours" {
				t.Errorf("field = %q, want quiet_hours", got)
			}
		})
	}
}

func TestPutPreferences_UnknownFieldIs422(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	body := `{"locale":"vi","theme":"dark","daily_goal_minutes":30,` +
		`"notification_channels":["in_app"],"ai_processing_opt_out":false,"dark_mode":true}`
	recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/preferences", body)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}
	if got := decodeProblem(t, recorder).Errors[0].Field; got != "dark_mode" {
		t.Errorf("field = %q, want dark_mode", got)
	}
}

func TestPreferences_WithoutAnActorIsUnauthenticated(t *testing.T) {
	t.Parallel()
	server := newServer(&fakeAccounts{})

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		body := ""
		if method == http.MethodPut {
			body = validPreferencesBody
		}
		recorder := do(t, server, method, "/api/v1/me/preferences", body, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", method, recorder.Code)
		}
	}
}
