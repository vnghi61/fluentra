package pagination

import (
	"encoding/base64"
	"testing"
	"time"

	"pgregory.net/rapid"
)

func TestCursor_RoundTripSupportedTypes(t *testing.T) {
	t.Parallel()
	stringCursor := Cursor[string]{SortValue: "lesson", ID: "id-1"}
	intCursor := Cursor[int]{SortValue: 42, ID: "id-2"}
	int64Cursor := Cursor[int64]{SortValue: 99, ID: "id-3"}
	timeCursor := Cursor[time.Time]{SortValue: time.Date(2026, 8, 7, 1, 2, 3, 4, time.UTC), ID: "id-4"}
	assertRoundTrip(t, stringCursor)
	assertRoundTrip(t, intCursor)
	assertRoundTrip(t, int64Cursor)
	assertRoundTrip(t, timeCursor)
}

func TestCursor_InvalidTypedValues(t *testing.T) {
	t.Parallel()
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	if _, err := Decode[int](encode(`{"t":"int","s":"wrong","i":"id"}`)); err == nil {
		t.Fatal("expected integer parse error")
	}
	if _, err := Decode[time.Time](encode(`{"t":"time","s":"wrong","i":"id"}`)); err == nil {
		t.Fatal("expected time parse error")
	}
	if _, err := Decode[time.Time](encode(`{"t":"string","s":"wrong","i":"id"}`)); err == nil {
		t.Fatal("expected time type error")
	}
	if _, err := Decode[string](encode(`{"t":"other","s":"x","i":"id"}`)); err == nil {
		t.Fatal("expected string type error")
	}
	if _, err := Decode[int](encode(`{"t":"string","s":"42","i":"id"}`)); err == nil {
		t.Fatal("expected integer type error")
	}
	if _, err := Decode[int64](encode(`{"t":"string","s":"42","i":"id"}`)); err == nil {
		t.Fatal("expected int64 type error")
	}
}

func TestCursor_RoundTripProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cursor := Cursor[string]{SortValue: rapid.String().Draw(t, "sort"), ID: rapid.String().Draw(t, "id")}
		encoded, err := Encode(cursor)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := Decode[string](encoded)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if decoded != cursor {
			t.Fatalf("decoded = %#v, want %#v", decoded, cursor)
		}
	})
}

func TestCursor_InvalidInput(t *testing.T) {
	t.Parallel()
	if _, err := Decode[string]("not-a-cursor"); err == nil {
		t.Fatal("expected invalid base64 error")
	}
	if _, err := Decode[int]("eyJ0Ijoic3RyaW5nIiwicyI6IngiLCJpIjoiaWQifQ"); err == nil {
		t.Fatal("expected type mismatch error")
	}
	if _, err := Decode[time.Time]("bm90LWpzb24"); err == nil {
		t.Fatal("expected malformed JSON error")
	}
	encoded, err := Encode(Cursor[int]{SortValue: 42, ID: "id"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode[int](encoded); err != nil {
		t.Fatalf("integer parse: %v", err)
	}
}

func assertRoundTrip[T SortValue](t *testing.T, cursor Cursor[T]) {
	t.Helper()
	encoded, err := Encode(cursor)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode[T](encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded != cursor {
		t.Fatalf("decoded = %#v, want %#v", decoded, cursor)
	}
}
