package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"slices"
	"testing"
	"time"
)

// Speaker labels and words the dialogue tests reuse.
const (
	speakerMan   = "Man"
	speakerWoman = "Woman"
	wordTwo      = "two"
)

func TestDialogueTurns(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		script string
		turns  []ScriptTurn
		want   []ScriptTurn
	}{
		"two speakers, names dropped from the words": {
			script: "Man: Is this seat free?\n\nWoman: Yes, please sit down.\nMan: Thanks.",
			want: []ScriptTurn{
				{Speaker: speakerMan, Text: "Is this seat free?"},
				{Speaker: speakerWoman, Text: "Yes, please sit down."},
				{Speaker: speakerMan, Text: "Thanks."},
			},
		},
		"multi-word and accented labels": {
			script: "Mr. Brown: Good morning.\nThu Hà: Chào thầy.",
			want:   []ScriptTurn{{Speaker: "Mr. Brown", Text: "Good morning."}, {Speaker: "Thu Hà", Text: "Chào thầy."}},
		},
		"one speaker is a monologue": {script: "Narrator: Welcome.\nNarrator: Listen carefully."},
		"a line without a speaker":   {script: "Man: Hello.\nThe phone rings.\nWoman: Hi."},
		"a time is not a speaker":    {script: "The train leaves at 10:30 sharp."},
		"plain prose":                {script: "Every morning I walk to the station."},
		"explicit turns win": {
			script: "irrelevant",
			turns:  []ScriptTurn{{Speaker: "A", Text: "one"}, {Speaker: "B", Text: wordTwo}},
			want:   []ScriptTurn{{Speaker: "A", Text: "one"}, {Speaker: "B", Text: wordTwo}},
		},
	}
	for name, tc := range cases {
		got := DialogueTurns(tc.script, tc.turns)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: DialogueTurns = %+v, want %+v", name, got, tc.want)
		}
	}
}

func TestDialogueVoiceKey(t *testing.T) {
	t.Parallel()
	if got := DialogueVoiceKey("a", "b"); got != "a+b" {
		t.Errorf("DialogueVoiceKey(a, b) = %q", got)
	}
	if got := DialogueVoiceKey("a", ""); got != "a+a" {
		t.Errorf("without a second voice = %q, want a+a", got)
	}
	if got := DialogueVoiceKey("", ""); got != DefaultVoice+"+"+DefaultVoice {
		t.Errorf("unconfigured = %q", got)
	}
}

// wavEngine renders every turn as a 16-bit mono WAV of len(text) samples and
// records what it was asked to say and in which voice.
type wavEngine struct {
	rate   uint32
	texts  []string
	voices []string
}

func (e *wavEngine) EngineName() string    { return "wav" }
func (e *wavEngine) EngineVersion() string { return "1" }
func (e *wavEngine) Render(_ context.Context, text, voice string) ([]byte, string, error) {
	e.texts = append(e.texts, text)
	e.voices = append(e.voices, voice)
	return testWAV(e.rate, bytes.Repeat([]byte{1, 0}, len(text))), mimeWAV, nil
}

func testWAV(rate uint32, data []byte) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	writeUint32(&b, uint32(4+24+8+len(data))) //nolint:gosec // G115: test clips are a few bytes
	b.WriteString("WAVEfmt ")
	writeUint32(&b, 16)
	_ = binary.Write(&b, binary.LittleEndian, []uint16{1, 1}) // PCM, mono
	writeUint32(&b, rate)
	writeUint32(&b, rate*2)
	_ = binary.Write(&b, binary.LittleEndian, []uint16{2, 16}) // block align, bits
	b.WriteString("data")
	writeUint32(&b, uint32(len(data))) //nolint:gosec // G115: test clips are a few bytes
	b.Write(data)
	return b.Bytes()
}

func TestRenderTurns_VoicesEachSpeakerAndJoinsOneValidWAV(t *testing.T) {
	t.Parallel()
	engine := &wavEngine{rate: 1000}
	turns := DialogueTurns("Man: Hi.\nWoman: Hello there.\nMan: Bye.", nil)

	audio, mime, err := RenderTurns(context.Background(), engine, turns, "a", "b")
	if err != nil {
		t.Fatalf("RenderTurns: %v", err)
	}
	if mime != mimeWAV {
		t.Fatalf("mime = %q", mime)
	}
	if want := []string{"a", "b", "a"}; !slices.Equal(engine.voices, want) {
		t.Errorf("voices = %v, want %v", engine.voices, want)
	}
	if want := []string{"Hi.", "Hello there.", "Bye."}; !slices.Equal(engine.texts, want) {
		t.Errorf("spoken = %q, want the words without the names", engine.texts)
	}

	part, err := parseWAV(audio)
	if err != nil {
		t.Fatalf("the joined clip is not one WAV: %v", err)
	}
	speech := 2 * (len("Hi.") + len("Hello there.") + len("Bye."))
	gap := int(int64(2000) * int64(turnGap) / int64(time.Second)) // byteRate 2000
	if want := speech + 2*gap; len(part.data) != want {
		t.Errorf("data = %d bytes, want %d (speech %d + two pauses of %d)", len(part.data), want, speech, gap)
	}
	if got := int(binary.LittleEndian.Uint32(audio[4:8])); got != len(audio)-8 {
		t.Errorf("RIFF size = %d, want %d", got, len(audio)-8)
	}
}

func TestJoinWAV_RefusesMixedSampleRates(t *testing.T) {
	t.Parallel()
	_, err := joinWAV([][]byte{testWAV(22050, []byte{0, 0}), testWAV(16000, []byte{0, 0})}, turnGap)
	if err == nil {
		t.Fatal("joining two sample rates must fail, not play at the wrong speed")
	}
}

func TestCacheLocator_PrefersTheTwoVoiceClipAndFallsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	script := "Man: Hi.\nWoman: Hello."
	cache := newFakeTTSCache()
	_ = cache.Put(ctx, HashText(script), "a", "", "", "tts/a/old.wav")
	locator := NewCacheLocator(cache).WithVoice("a").WithSecondVoice("b")

	key, found, err := locator.AudioKey(ctx, script, "")
	if err != nil || !found || key != "tts/a/old.wav" {
		t.Fatalf("before the two-voice render: %q %v %v, want the one-voice clip", key, found, err)
	}

	_ = cache.Put(ctx, HashText(script), "a+b", "", "", "tts/a+b/new.wav")
	key, found, err = locator.AudioKey(ctx, script, "")
	if err != nil || !found || key != "tts/a+b/new.wav" {
		t.Fatalf("after: %q %v %v, want the two-voice clip", key, found, err)
	}
}

func TestCachedSynthesiser_FindsADialogueScriptWithoutAnEngine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	script := "Man: Hi.\nWoman: Hello."
	cache := newFakeTTSCache()
	s := NewCachedSynthesiser(cache, nil, nil, "").WithVoice("a").WithSecondVoice("b")

	if _, err := s.Synthesise(ctx, script, ""); err == nil {
		t.Fatal("nothing cached and no engine must be not found")
	}
	_ = cache.Put(ctx, HashText(script), "a", "", "", "tts/a/old.wav")
	if key, err := s.Synthesise(ctx, script, ""); err != nil || key != "tts/a/old.wav" {
		t.Fatalf("one-voice clip: %q %v", key, err)
	}
	_ = cache.Put(ctx, HashText(script), "a+b", "", "", "tts/a+b/new.wav")
	if key, err := s.Synthesise(ctx, script, ""); err != nil || key != "tts/a+b/new.wav" {
		t.Fatalf("two-voice clip: %q %v", key, err)
	}
}

func TestPiperEngine_PassesPaceAndPauses(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	engine := NewPiperEngine("piper", "/models", "1")
	engine.LengthScale = 1.25
	engine.SentenceSilence = 0.35
	engine.run = func(_ context.Context, _ string, args []string, _ io.Reader) error {
		gotArgs = args
		return os.WriteFile(args[3], testWAV(22050, []byte{0, 0}), 0o600)
	}
	if _, _, err := engine.Render(context.Background(), "Slowly.", "v"); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range [][2]string{{"--length_scale", "1.25"}, {"--sentence_silence", "0.35"}} {
		i := slices.Index(gotArgs, want[0])
		if i < 0 || i+1 >= len(gotArgs) || gotArgs[i+1] != want[1] {
			t.Errorf("args %v lack %s %s", gotArgs, want[0], want[1])
		}
	}
}
