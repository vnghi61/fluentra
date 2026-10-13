package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func items(ids ...string) []listeningItem {
	out := make([]listeningItem, 0, len(ids))
	for _, id := range ids {
		out = append(out, listeningItem{ID: id, Body: listeningBody{Script: "say " + id}})
	}
	return out
}

func TestRenderItems_RendersEveryItemWithoutABudget(t *testing.T) {
	var rendered []string
	var out bytes.Buffer
	summary := renderItems(items("a", "b", "c"), func(item listeningItem) (string, bool, error) {
		rendered = append(rendered, item.ID)
		return "tts/" + item.ID, false, nil
	}, time.Time{}, time.Now, &out)

	if summary != (renderSummary{Rendered: 3}) {
		t.Fatalf("summary = %+v, want 3 rendered", summary)
	}
	if strings.Join(rendered, ",") != "a,b,c" {
		t.Fatalf("rendered %v, want a,b,c in order", rendered)
	}
}

// A failed item is a warning, not the end of the run: the old loop survived
// these too, and the next item still renders.
func TestRenderItems_KeepsGoingPastAFailedItem(t *testing.T) {
	var out bytes.Buffer
	summary := renderItems(items("a", "b", "c"), func(item listeningItem) (string, bool, error) {
		if item.ID == "b" {
			return "", false, errors.New("piper exited 1")
		}
		return "tts/" + item.ID, false, nil
	}, time.Time{}, time.Now, &out)

	if summary != (renderSummary{Rendered: 2, Failed: 1}) {
		t.Fatalf("summary = %+v, want 2 rendered and 1 failed", summary)
	}
	if !strings.Contains(out.String(), "failed to synthesise item b: piper exited 1") {
		t.Fatalf("output does not name the failed item:\n%s", out.String())
	}
}

// The budget stops new work before the job timeout would kill the run half way
// through an upload, and says how much is left.
func TestRenderItems_StopsAtTheDeadline(t *testing.T) {
	start := time.Date(2026, 10, 13, 2, 0, 0, 0, time.UTC)
	clock := start
	now := func() time.Time { return clock }
	deadline := start.Add(10 * time.Minute)

	var out bytes.Buffer
	summary := renderItems(items("a", "b", "c", "d"), func(item listeningItem) (string, bool, error) {
		clock = clock.Add(6 * time.Minute) // one render
		return "tts/" + item.ID, false, nil
	}, deadline, now, &out)

	// a starts at 0m, b at 6m, c would start at 12m: past the deadline.
	if summary != (renderSummary{Rendered: 2, Remaining: 2}) {
		t.Fatalf("summary = %+v, want 2 rendered and 2 remaining", summary)
	}
	if !strings.Contains(out.String(), "2 item(s) left for the next run") {
		t.Fatalf("output does not report what is left:\n%s", out.String())
	}
}

// A clip already in the cache counts as voiced but is not rendered again, and
// the job summary adds the four numbers up to the catalogue.
func TestRenderItems_CountsCachedClipsAndSummarises(t *testing.T) {
	var out bytes.Buffer
	summary := renderItems(items("a", "b", "c"), func(item listeningItem) (string, bool, error) {
		return "tts/" + item.ID, item.ID != "c", nil
	}, time.Time{}, time.Now, &out)

	if summary != (renderSummary{Cached: 2, Rendered: 1}) {
		t.Fatalf("summary = %+v, want 2 cached and 1 rendered", summary)
	}
	if strings.Contains(out.String(), "item a") {
		t.Fatalf("a cached item was logged as work:\n%s", out.String())
	}

	md := renderSummary{Cached: 150, Rendered: 50, Failed: 2, Remaining: 198}.Markdown()
	if !strings.Contains(md, "**200 of 400** listening items have a clip (50%)") {
		t.Fatalf("summary markdown:\n%s", md)
	}
}
