package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func TestDecodeJSON_UnknownField_ReturnsBadRequest(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Ada","unexpected":true}`))
	var body struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(request, &body)
	if !apperr.Is(err, apperr.BadRequest) {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteProblem_DoesNotExposeCause(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("GET", "/private", nil)
	request = request.WithContext(WithRequestID(request.Context(), "req-1"))
	response := httptest.NewRecorder()
	WriteProblem(response, request, apperr.New(apperr.Internal, "INTERNAL_ERROR", "safe message").WithInternal("secret"))
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("response leaked detail: %s", response.Body.String())
	}
}

func TestDecodeJSON_TrailingDataAndTooLarge(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Ada"} {}`))
	var body struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(request, &body); !apperr.Is(err, apperr.BadRequest) {
		t.Fatalf("trailing error = %v", err)
	}
	request = httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Ada"}`))
	if err := DecodeJSONLimit(request, &body, 4); !apperr.Is(err, apperr.TooLarge) {
		t.Fatalf("too large error = %v", err)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(WithRequestID(request.Context(), "request-1"))
	response := httptest.NewRecorder()
	WriteJSON(response, request, 201, map[string]string{"status": "created"})
	if response.Code != 201 ||
		response.Header().Get("X-Request-Id") != "request-1" ||
		!strings.Contains(response.Body.String(), "created") {
		t.Fatalf("unexpected response: %#v, %s", response.Result(), response.Body.String())
	}
}
