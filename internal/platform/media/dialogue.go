package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// turnGap is the silence between two turns of a conversation.
const turnGap = 600 * time.Millisecond

// speakerLine matches "Speaker: words". The label starts with a letter and stays
// short, so a sentence holding a colon ("Meet at 10:30") is not split.
var speakerLine = regexp.MustCompile(`^(\p{L}[\p{L}\p{N} .'\-]{0,29}?)\s*:\s+(\S.*)$`)

// DialogueTurns returns the turns a listening script is voiced as: the body's own
// turns when it has two or more; otherwise its "Speaker: line" lines, when every
// line has a speaker and at least two speakers talk. Anything else is a
// monologue and returns nil. The speaker's name is never part of a turn's text,
// so it is not read aloud.
func DialogueTurns(script string, turns []ScriptTurn) []ScriptTurn {
	if len(turns) >= 2 {
		return turns
	}
	var parsed []ScriptTurn
	speakers := map[string]struct{}{}
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		match := speakerLine.FindStringSubmatch(line)
		if match == nil {
			return nil
		}
		name := strings.TrimSpace(match[1])
		speakers[strings.ToLower(name)] = struct{}{}
		parsed = append(parsed, ScriptTurn{Speaker: name, Text: strings.TrimSpace(match[2])})
	}
	if len(speakers) < 2 {
		return nil
	}
	return parsed
}

// DialogueVoiceKey is the voice a conversation clip is cached and stored under:
// voice A, then voice B (voice A again when no second voice is configured).
func DialogueVoiceKey(voiceA, voiceB string) string {
	a := ConfiguredVoice(voiceA)
	return a + "+" + ConfiguredVoice(firstNonBlank(voiceB, a))
}

// RenderTurns renders a conversation: each turn's words in its speaker's voice
// (the first speaker heard is voice A, everyone else voice B), joined with a
// pause. WAV turns are joined into one valid file.
func RenderTurns(
	ctx context.Context, engine SynthesiserEngine, turns []ScriptTurn, voiceA, voiceB string,
) ([]byte, string, error) {
	if len(turns) == 0 {
		return nil, "", errors.New("render turns: no turns")
	}
	a := ConfiguredVoice(voiceA)
	b := ConfiguredVoice(firstNonBlank(voiceB, a))
	first := strings.TrimSpace(turns[0].Speaker)

	parts := make([][]byte, 0, len(turns))
	mimeType := ""
	for _, turn := range turns {
		text := strings.TrimSpace(turn.Text)
		if text == "" {
			continue
		}
		voice := b
		if strings.EqualFold(strings.TrimSpace(turn.Speaker), first) {
			voice = a
		}
		data, mime, err := engine.Render(ctx, text, voice)
		if err != nil {
			return nil, "", fmt.Errorf("render turn: %w", err)
		}
		if mimeType == "" {
			mimeType = mime
		}
		parts = append(parts, data)
	}
	if len(parts) == 0 {
		return nil, "", errors.New("render turns: every turn is empty")
	}
	if mimeType == mimeWAV {
		joined, err := joinWAV(parts, turnGap)
		if err != nil {
			return nil, "", err
		}
		return joined, mimeWAV, nil
	}
	if mimeType == "" {
		mimeType = mimeAudioMPEG
	}
	return bytes.Join(parts, nil), mimeType, nil
}
