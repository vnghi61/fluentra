package secret

import (
	"encoding/json"
	"testing"
)

func TestRedacted_StringAndReveal(t *testing.T) {
	t.Parallel()
	value := New("sensitive-token")
	if got := value.String(); got != "[redacted]" {
		t.Fatalf("String() = %q", got)
	}
	if got := value.Reveal(); got != "sensitive-token" {
		t.Fatalf("Reveal() = %q", got)
	}
}

func TestRedacted_MarshalJSON(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(New("sensitive-token"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(body) != `"[redacted]"` {
		t.Fatalf("body = %s", body)
	}
}
