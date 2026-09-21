package domain

import (
	"encoding/json"
	"testing"
)

func TestNormaliseText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Hello, World!  ", "hello world"},
		{"She ___ to Paris last year.", "she to paris last year"},
		{"Don't count your chickens before they hatch!", "dont count your chickens before they hatch"},
		{"Multiple   spaces \t and \n newlines...", "multiple spaces and newlines"},
		{"", ""},
	}

	for _, tc := range tests {
		got := NormaliseText(tc.input)
		if got != tc.expected {
			t.Errorf("NormaliseText(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestComputeFingerprint_OptionOrderInvariance(t *testing.T) {
	const optGoes = "goes"
	stem := "Choose the best answer for the blank: She ___ to school."
	opts1 := []string{"went", optGoes, "has gone", "going"}
	opts2 := []string{"going", "has gone", "went", optGoes}
	opts3 := []string{"  WENT  ", optGoes, "Going", "has gone!"}

	fp1 := ComputeFingerprint(stem, opts1)
	fp2 := ComputeFingerprint(stem, opts2)
	fp3 := ComputeFingerprint(stem, opts3)

	if fp1 != fp2 {
		t.Errorf("expected fp1 == fp2, got %s vs %s", fp1, fp2)
	}
	if fp1 != fp3 {
		t.Errorf("expected fp1 == fp3, got %s vs %s", fp1, fp3)
	}
}

func TestFingerprintFromBody_McqGap(t *testing.T) {
	body1 := []byte(`{
		"sentence": "Ms. Tanaka requested that the financial summary be completed _____ Friday morning.",
		"options": [
			{"id": "A", "text": "by"},
			{"id": "B", "text": "during"},
			{"id": "C", "text": "between"},
			{"id": "D", "text": "along"}
		],
		"correct_answer": "A"
	}`)

	body2 := []byte(`{
		"sentence": "  ms. tanaka requested that the financial summary be completed ___ Friday morning.  ",
		"options": [
			{"id": "1", "text": "during"},
			{"id": "2", "text": "along"},
			{"id": "3", "text": "by"},
			{"id": "4", "text": "between"}
		],
		"correct_answer": "3"
	}`)

	fp1, err := FingerprintFromBody("mcq_gap", json.RawMessage(body1))
	if err != nil {
		t.Fatalf("FingerprintFromBody failed: %v", err)
	}

	fp2, err := FingerprintFromBody("mcq_gap", json.RawMessage(body2))
	if err != nil {
		t.Fatalf("FingerprintFromBody failed: %v", err)
	}

	if fp1 != fp2 {
		t.Errorf("expected identical fingerprints, got %s vs %s", fp1, fp2)
	}
}

func TestFingerprintFromBody_MultiQuestion(t *testing.T) {
	body := []byte(`{
		"passage": "A major corporation announced quarterly earnings yesterday...",
		"questions": [
			{
				"id": "q1",
				"prompt": "What was announced?",
				"options": [
					{"id": "A", "text": "Earnings"},
					{"id": "B", "text": "Merger"}
				]
			},
			{
				"id": "q2",
				"prompt": "When was it announced?",
				"options": [
					{"id": "A", "text": "Yesterday"},
					{"id": "B", "text": "Today"}
				]
			}
		]
	}`)

	fp, err := FingerprintFromBody("reading_comprehension", json.RawMessage(body))
	if err != nil {
		t.Fatalf("FingerprintFromBody failed: %v", err)
	}
	if fp == "" {
		t.Error("expected non-empty fingerprint")
	}
}
