package ai

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// mockItemSequence numbers the items the mock generates for the practice and
// exam pools. It starts from the clock, so a restarted worker does not number
// from 1 again and write items already in a slot.
var mockItemSequence atomic.Int64

// The texts the pools compare to refuse a duplicate.
const (
	mockFieldPassage       = "passage"
	mockFieldPrompt        = "prompt"
	mockFieldReferenceText = "reference_text"
	mockFieldScript        = "script"
)

var mockDistinctFields = []string{mockFieldPassage, mockFieldPrompt, mockFieldReferenceText, mockFieldScript}

// distinctMockItem numbers a generated item so it is its own.
//
// The practice and exam pools refuse an item whose prompt, passage, script or
// reading text matches one already in its slot. The mock returned the same item
// on every call, so each slot kept exactly one, and a sitting needs two or
// three: on a development stack the exam hub never had an exam to offer.
func distinctMockItem(resp Response, err error) (Response, error) {
	if err != nil {
		return resp, err
	}
	// A reply that is not a JSON object has nothing to number and is passed on.
	var item map[string]any
	if json.Unmarshal([]byte(resp.Text), &item) == nil && item != nil {
		resp.Text, err = numberMockItem(item)
	}
	return resp, err
}

func numberMockItem(item map[string]any) (string, error) {
	mockItemSequence.CompareAndSwap(0, time.Now().Unix())
	n := mockItemSequence.Add(1)
	for _, field := range mockDistinctFields {
		if text, ok := item[field].(string); ok && text != "" {
			item[field] = fmt.Sprintf("[%d] %s", n, text)
		}
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		return "", fmt.Errorf("ai: encode mock item: %w", err)
	}
	return string(encoded), nil
}
