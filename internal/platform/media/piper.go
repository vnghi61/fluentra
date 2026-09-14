package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnginePiper names the Piper engine.
const EnginePiper = "piper"

const mimeWAV = "audio/wav"

// PiperEngine renders speech with the Piper command-line synthesiser on CPU.
//
// Work order 12 §4 found that no engine fits the free worker, so Piper runs
// offline, in cmd/tts, on the owner's machine or a CI job. A voice is a model
// file: `<ModelDir>/<voice>.onnx`, with its `.onnx.json` beside it.
type PiperEngine struct {
	Binary   string
	ModelDir string
	Version  string

	// run executes the binary. Tests replace it; production uses os/exec.
	run func(ctx context.Context, name string, args []string, stdin io.Reader) error
}

// NewPiperEngine builds a Piper engine. An empty binary means `piper` on PATH.
func NewPiperEngine(binary, modelDir, version string) *PiperEngine {
	if binary == "" {
		binary = EnginePiper
	}
	return &PiperEngine{Binary: binary, ModelDir: modelDir, Version: version, run: execPiper}
}

// EngineName returns the engine name recorded in the TTS cache.
func (p *PiperEngine) EngineName() string { return EnginePiper }

// EngineVersion returns the engine version recorded in the TTS cache.
func (p *PiperEngine) EngineVersion() string {
	if p.Version == "" {
		return "unknown"
	}
	return p.Version
}

// Render synthesises text in voice and returns WAV bytes.
func (p *PiperEngine) Render(ctx context.Context, text, voice string) ([]byte, string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, "", errors.New("piper: nothing to render")
	}
	if voice == "" {
		voice = DefaultVoice
	}
	// A voice names a file in ModelDir and nothing else.
	if strings.ContainsAny(voice, `/\`) || strings.Contains(voice, "..") {
		return nil, "", fmt.Errorf("piper: invalid voice %q", voice)
	}
	if p.ModelDir == "" {
		return nil, "", errors.New("piper: no model directory")
	}

	dir, err := os.MkdirTemp("", "fluentra-tts-")
	if err != nil {
		return nil, "", fmt.Errorf("piper: temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	outPath := filepath.Join(dir, "clip.wav")
	args := []string{"--model", filepath.Join(p.ModelDir, voice+".onnx"), "--output_file", outPath}
	run := p.run
	if run == nil {
		run = execPiper
	}
	if err := run(ctx, p.Binary, args, strings.NewReader(text)); err != nil {
		return nil, "", fmt.Errorf("piper: %w", err)
	}

	audio, err := os.ReadFile(outPath) //nolint:gosec // G304: a path inside the temp dir this call created
	if err != nil {
		return nil, "", fmt.Errorf("piper: read output: %w", err)
	}
	if len(audio) == 0 {
		return nil, "", errors.New("piper: empty output")
	}
	return audio, mimeWAV, nil
}

func execPiper(ctx context.Context, name string, args []string, stdin io.Reader) error {
	//nolint:gosec // G204: the binary is the operator's own -piper flag to an offline command
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// ObjectKey is where a rendered clip is stored: by voice and text hash, with
// the extension of its format.
func ObjectKey(voice, textHash, mimeType string) string {
	ext := ".mp3"
	switch mimeType {
	case mimeWAV, "audio/x-wav":
		ext = ".wav"
	case "audio/ogg":
		ext = ".ogg"
	}
	return fmt.Sprintf("tts/%s/%s%s", voice, textHash, ext)
}
