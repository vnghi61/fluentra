package media

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPiperEngine_RendersThroughTheBinaryIntoATempFile(t *testing.T) {
	var gotArgs []string
	var gotText string
	engine := NewPiperEngine("/opt/piper/piper", "/models", "1.2.0")
	engine.run = func(_ context.Context, name string, args []string, stdin io.Reader) error {
		if name != "/opt/piper/piper" {
			t.Fatalf("binary = %q", name)
		}
		gotArgs = args
		text, _ := io.ReadAll(stdin)
		gotText = string(text)
		return os.WriteFile(args[3], []byte("RIFF....WAVE"), 0o600)
	}

	audio, mime, err := engine.Render(context.Background(), "Good morning.", "en_US-amy-medium")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if mime != mimeWAV || string(audio) != "RIFF....WAVE" {
		t.Fatalf("got %q %q", mime, audio)
	}
	if gotText != "Good morning." {
		t.Fatalf("stdin = %q", gotText)
	}
	if gotArgs[0] != "--model" || gotArgs[1] != filepath.Join("/models", "en_US-amy-medium.onnx") {
		t.Fatalf("args = %v", gotArgs)
	}
	if _, err := os.Stat(gotArgs[3]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp output was not removed: %v", err)
	}
}

func TestPiperEngine_RefusesAVoiceThatIsAPath(t *testing.T) {
	engine := NewPiperEngine("", "/models", "")
	engine.run = func(context.Context, string, []string, io.Reader) error {
		t.Fatal("the binary must not run")
		return nil
	}
	for _, voice := range []string{"../etc/passwd", "a/b", `a\b`} {
		if _, _, err := engine.Render(context.Background(), "text", voice); err == nil {
			t.Fatalf("voice %q was accepted", voice)
		}
	}
}

func TestPiperEngine_AFailedRunIsAnError(t *testing.T) {
	engine := NewPiperEngine("", "/models", "")
	engine.run = func(context.Context, string, []string, io.Reader) error { return errors.New("model not found") }
	if _, _, err := engine.Render(context.Background(), "text", ""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestObjectKey_FollowsTheFormat(t *testing.T) {
	if got := ObjectKey("v", "abc", "audio/wav"); got != "tts/v/abc.wav" {
		t.Fatalf("got %q", got)
	}
	if got := ObjectKey("v", "abc", "audio/mpeg"); got != "tts/v/abc.mp3" {
		t.Fatalf("got %q", got)
	}
}
