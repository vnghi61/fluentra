package main

import "testing"

// TestFixtureDecksFor is D22-10: a fixture word goes to its level's deck and,
// when it is among the commonest thousand, to "Top 1,000" — never to one deck
// of all 10,000, and never to the curated 200's.
func TestFixtureDecksFor(t *testing.T) {
	common := fixtureDecksFor(seedWordSense{Lemma: "time", CEFRLevel: "A1", Rank: 42})
	if len(common) != 2 || common[0].Slug != "words-a1" || common[1].Slug != topWordsDeck.Slug {
		t.Errorf("a top-1,000 A1 word goes to %+v, want words-a1 and top-1000", common)
	}

	rare := fixtureDecksFor(seedWordSense{Lemma: "therapeutic", CEFRLevel: "C1", Rank: 9001})
	if len(rare) != 1 || rare[0].Slug != "words-c1" {
		t.Errorf("a rare C1 word goes to %+v, want words-c1 only", rare)
	}

	unranked := fixtureDecksFor(seedWordSense{Lemma: "time", CEFRLevel: "A1"})
	if len(unranked) != 1 {
		t.Errorf("an unranked word goes to %+v, want its level deck only", unranked)
	}
	for _, deck := range append(append(common, rare...), unranked...) {
		if deck.Slug == curatedDeck.Slug {
			t.Error("a fixture word must not join the curated 200's deck")
		}
	}
}
