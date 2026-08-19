package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const allowOrigin = "http://localhost:5173"

func TestCORS_AllowsAnAllowlistedOrigin(t *testing.T) {
	handler := CORS([]string{allowOrigin})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Origin", allowOrigin)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != allowOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want the echoed origin", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestCORS_LeavesAnUnlistedOriginWithNoHeaders(t *testing.T) {
	handler := CORS([]string{allowOrigin})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want unset for an unlisted origin", got)
	}
}

func TestCORS_AnswersAPreflightWithoutReachingTheHandler(t *testing.T) {
	reached := false
	handler := CORS([]string{allowOrigin})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		reached = true
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/refresh", nil)
	request.Header.Set("Origin", allowOrigin)
	request.Header.Set("Access-Control-Request-Method", "POST")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if reached {
		t.Error("the preflight reached the handler instead of being answered by the middleware")
	}
	if got := recorder.Code; got != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", got, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("preflight did not set Access-Control-Allow-Methods")
	}
}

func TestCORS_IsANoOpWithoutAnOriginHeader(t *testing.T) {
	handler := CORS([]string{allowOrigin})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want unset for a same-origin request", got)
	}
}
