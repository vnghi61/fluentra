package media

import (
	"context"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/storage"
)

// The two voices the conversation tests render in.
const (
	testVoiceA = "voiceA"
	testVoiceB = "voiceB"
)

// recordingEngine records the voice each turn was rendered in and returns the
// turn's text as its "audio", so concatenation is easy to check.
type recordingEngine struct {
	voices []string
}

func (e *recordingEngine) EngineName() string    { return "recording" }
func (e *recordingEngine) EngineVersion() string { return "1" }
func (e *recordingEngine) Render(_ context.Context, text, voice string) ([]byte, string, error) {
	e.voices = append(e.voices, voice)
	return []byte(text), mimeAudioMPEG, nil
}

func TestCachedSynthesiser_SynthesiseTurnsVoicesEachSpeaker(t *testing.T) {
	engine := &recordingEngine{}
	uploads := newFakeStorage()
	s := NewCachedSynthesiser(nil, engine, uploads, "").
		WithVoice("voiceA").
		WithSecondVoice("voiceB")

	key, err := s.SynthesiseTurns(context.Background(), []ScriptTurn{
		{Speaker: "Woman", Text: "Hello."},
		{Speaker: "Man", Text: "Hi."},
		{Speaker: "Woman", Text: "How are you?"},
	})
	if err != nil {
		t.Fatalf("render conversation: %v", err)
	}
	if key == "" {
		t.Fatal("a conversation must produce an object key")
	}
	want := []string{testVoiceA, testVoiceB, testVoiceA}
	if len(engine.voices) != len(want) {
		t.Fatalf("voices = %v, want %v", engine.voices, want)
	}
	for i := range want {
		if engine.voices[i] != want[i] {
			t.Errorf("turn %d voice = %q, want %q", i, engine.voices[i], want[i])
		}
	}

	// The three turns are one clip, concatenated in order.
	got := string(uploads.uploads[storage.BucketMedia+"/"+key])
	wantClip := "Hello.Hi.How are you?"
	if got != wantClip {
		t.Errorf("clip = %q, want %q", got, wantClip)
	}
}

func TestCachedSynthesiser_SynthesiseTurnsFallsBackForOneTurn(t *testing.T) {
	engine := &recordingEngine{}
	uploads := newFakeStorage()
	s := NewCachedSynthesiser(nil, engine, uploads, "").WithVoice("voiceA")

	if _, err := s.SynthesiseTurns(context.Background(), []ScriptTurn{
		{Speaker: "Narrator", Text: "A short announcement."},
	}); err != nil {
		t.Fatalf("a single turn must render: %v", err)
	}
	if len(engine.voices) != 1 || engine.voices[0] != testVoiceA {
		t.Errorf("voices = %v, want one voiceA", engine.voices)
	}
}

func TestParseScriptTurns(t *testing.T) {
	turns := ParseScriptTurns([]byte(`{"title":"x","turns":[{"speaker":"A","text":"one"},{"speaker":"B","text":"two"}]}`))
	if len(turns) != 2 || turns[0].Speaker != "A" || turns[1].Text != "two" {
		t.Fatalf("turns = %+v, want two", turns)
	}
	if got := ParseScriptTurns([]byte(`{"title":"x"}`)); got != nil {
		t.Errorf("a body without turns = %+v, want nil", got)
	}
	if got := ParseScriptTurns(nil); got != nil {
		t.Errorf("an empty body = %+v, want nil", got)
	}
}
