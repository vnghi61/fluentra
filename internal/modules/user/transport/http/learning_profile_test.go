package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

const validLearningProfileBody = `{
	"declared_level": "B1",
	"target_level": "B2",
	"target_exam": "ielts",
	"weekly_minutes_goal": 150,
	"motivations": ["career", "education"]
}`

func TestGetLearningProfile_Unauthenticated(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := anonymous(t, server, http.MethodGet, "/api/v1/me/learning-profile", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestGetLearningProfile_NotFound(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{err: domain.ErrLearningProfileNotFound}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/me/learning-profile", "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", recorder.Code, recorder.Body)
	}
	prob := decodeProblem(t, recorder)
	if prob.Code != "LEARNING_PROFILE_NOT_FOUND" {
		t.Errorf("code = %q, want LEARNING_PROFILE_NOT_FOUND", prob.Code)
	}
}

func TestGetLearningProfile_Success(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/me/learning-profile", "")
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
		"declared_level", "target_level", "target_exam",
		"weekly_minutes_goal", "motivations", "created_at", "updated_at",
	} {
		if _, present := body[field]; !present {
			t.Errorf("response is missing %q", field)
		}
	}
	if body["declared_level"] != "B1" {
		t.Errorf("declared_level = %v, want B1", body["declared_level"])
	}
	if body["target_exam"] != "ielts" {
		t.Errorf("target_exam = %v, want ielts", body["target_exam"])
	}
}

func TestReplaceLearningProfile_ValidatesAndReplaces(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/learning-profile", validLearningProfileBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	if accounts.seenActor != actorID {
		t.Errorf("service was asked for %s, want %s", accounts.seenActor, actorID)
	}
	if accounts.seenLearningProfile.TargetExam != domain.TargetExamIELTS {
		t.Errorf("seen TargetExam = %v, want ielts", accounts.seenLearningProfile.TargetExam)
	}
	if accounts.seenLearningProfile.DeclaredLevel == nil || *accounts.seenLearningProfile.DeclaredLevel != "B1" {
		t.Errorf("seen DeclaredLevel = %v, want B1", accounts.seenLearningProfile.DeclaredLevel)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["target_exam"] != "ielts" {
		t.Errorf("response target_exam = %v, want ielts", body["target_exam"])
	}
}

func TestReplaceLearningProfile_RejectsUnknownField(t *testing.T) {
	t.Parallel()
	accounts := &fakeAccounts{}
	server := newServer(accounts)

	body := `{
		"declared_level": "B1",
		"target_level": "B2",
		"target_exam": "ielts",
		"weekly_minutes_goal": 150,
		"motivations": ["career"],
		"unknown_field": "val"
	}`

	recorder := authenticated(t, server, http.MethodPut, "/api/v1/me/learning-profile", body)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	prob := decodeProblem(t, recorder)
	found := false
	for _, errItem := range prob.Errors {
		if errItem.Field == "unknown_field" && errItem.Code == "UNKNOWN_FIELD" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNKNOWN_FIELD for unknown_field, got: %+v", prob.Errors)
	}
}
