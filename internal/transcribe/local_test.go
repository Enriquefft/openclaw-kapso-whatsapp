package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
)

// testExecCmd builds an injectable execCmd that:
// - For ffmpeg: calls ffmpegSide for side effects (e.g. write WAV), returns exit code ffmpegExit.
// - For whisper: calls whisperSide for side effects (e.g. write txt), returns exit code whisperExit.
//
// Side effects run before the command, using a real "true"/"false" subprocess to
// produce the correct exit code. File writes happen in the side effect functions.
func testExecCmd(
	ffmpegSide func(args []string),
	ffmpegFail bool,
	whisperSide func(args []string),
	whisperFail bool,
) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		isFfmpeg := name == "ffmpeg"
		if isFfmpeg {
			if ffmpegSide != nil {
				ffmpegSide(args)
			}
			if ffmpegFail {
				return exec.CommandContext(ctx, "false")
			}
			return exec.CommandContext(ctx, "true")
		}
		// whisper-cli
		if whisperSide != nil {
			whisperSide(args)
		}
		if whisperFail {
			return exec.CommandContext(ctx, "false")
		}
		return exec.CommandContext(ctx, "true")
	}
}

// findArgAfter returns the value of the argument immediately following the given flag in args.
func findArgAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestLocalWhisper(t *testing.T) {
	fakeAudio := []byte("fake-ogg-audio-bytes")
	transcript := "  hello local whisper  \n"

	tests := []struct {
		name               string
		cfg                config.TranscribeConfig
		wantNewErr         bool
		wantNewErrContains string

		ffmpegFail  bool
		whisperFail bool

		wantTranscribeErr         bool
		wantTranscribeErrContains string
		wantText                  string
	}{
		{
			name: "newLocalWhisper empty ModelPath returns error",
			cfg: config.TranscribeConfig{
				BinaryPath: "whisper-cli",
				ModelPath:  "",
			},
			wantNewErr:         true,
			wantNewErrContains: "model_path",
		},
		{
			name: "Transcribe success returns trimmed transcript",
			cfg: config.TranscribeConfig{
				BinaryPath: "whisper-cli",
				ModelPath:  "/models/ggml-base.bin",
				Language:   "",
			},
			wantText: strings.TrimSpace(transcript),
		},
		{
			name: "Transcribe ffmpeg failure returns error containing ffmpeg",
			cfg: config.TranscribeConfig{
				BinaryPath: "whisper-cli",
				ModelPath:  "/models/ggml-base.bin",
			},
			ffmpegFail:                true,
			wantTranscribeErr:         true,
			wantTranscribeErrContains: "ffmpeg",
		},
		{
			name: "Transcribe whisper-cli failure returns error containing whisper-cli",
			cfg: config.TranscribeConfig{
				BinaryPath: "whisper-cli",
				ModelPath:  "/models/ggml-base.bin",
			},
			whisperFail:               true,
			wantTranscribeErr:         true,
			wantTranscribeErrContains: "whisper-cli",
		},
		{
			name: "Transcribe with Language includes -l flag",
			cfg: config.TranscribeConfig{
				BinaryPath: "whisper-cli",
				ModelPath:  "/models/ggml-base.bin",
				Language:   "es",
			},
			wantText: strings.TrimSpace(transcript),
		},
		{
			name: "Transcribe with empty Language excludes -l flag",
			cfg: config.TranscribeConfig{
				BinaryPath: "whisper-cli",
				ModelPath:  "/models/ggml-base.bin",
				Language:   "",
			},
			wantText: strings.TrimSpace(transcript),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test construction.
			p, err := newLocalWhisper(tc.cfg)
			if tc.wantNewErr {
				if err == nil {
					t.Fatal("expected error from newLocalWhisper, got nil")
				}
				if tc.wantNewErrContains != "" && !strings.Contains(err.Error(), tc.wantNewErrContains) {
					t.Errorf("newLocalWhisper error %q does not contain %q", err.Error(), tc.wantNewErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("newLocalWhisper unexpected error: %v", err)
			}

			// Capture whisper args to check language flag.
			var capturedWhisperArgs []string

			// Wire mock execCmd.
			p.execCmd = testExecCmd(
				// ffmpeg side effect: write a dummy WAV file to the output path.
				func(args []string) {
					// ffmpeg args: -y -loglevel error -i <input> -acodec pcm_s16le -ac 1 -ar 16000 <output>
					// output is the last argument.
					if len(args) > 0 {
						wavPath := args[len(args)-1]
						_ = os.WriteFile(wavPath, []byte("dummy-wav"), 0o600)
					}
				},
				tc.ffmpegFail,
				// whisper-cli side effect: write transcript to -of prefix + ".txt".
				func(args []string) {
					capturedWhisperArgs = args
					prefix := findArgAfter(args, "-of")
					if prefix != "" {
						txtPath := prefix + ".txt"
						_ = os.WriteFile(txtPath, []byte(transcript), 0o600)
					}
				},
				tc.whisperFail,
			)

			ctx := context.Background()
			got, err := p.Transcribe(ctx, fakeAudio, "audio/ogg")

			if tc.wantTranscribeErr {
				if err == nil {
					t.Fatal("expected error from Transcribe, got nil")
				}
				if tc.wantTranscribeErrContains != "" && !strings.Contains(err.Error(), tc.wantTranscribeErrContains) {
					t.Errorf("Transcribe error %q does not contain %q", err.Error(), tc.wantTranscribeErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Transcribe unexpected error: %v", err)
			}
			if got != tc.wantText {
				t.Errorf("Transcribe() = %q, want %q", got, tc.wantText)
			}

			// Check language flag presence.
			if tc.cfg.Language != "" {
				// Should include "-l" and the language value.
				langVal := findArgAfter(capturedWhisperArgs, "-l")
				if langVal != tc.cfg.Language {
					t.Errorf("whisper-cli args: -l = %q, want %q", langVal, tc.cfg.Language)
				}
			} else {
				// Should NOT include "-l".
				for _, a := range capturedWhisperArgs {
					if a == "-l" {
						t.Error("whisper-cli args contain -l flag but Language is empty")
						break
					}
				}
			}
		})
	}
}

func TestLocalWhisperTempCleanup(t *testing.T) {
	// Verify that the temp directory is cleaned up after Transcribe returns.
	cfg := config.TranscribeConfig{
		BinaryPath: "whisper-cli",
		ModelPath:  "/models/ggml-base.bin",
	}
	p, err := newLocalWhisper(cfg)
	if err != nil {
		t.Fatalf("newLocalWhisper: %v", err)
	}

	var capturedDir string

	p.execCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Capture the temp dir from the first arg path.
		if capturedDir == "" && len(args) > 0 {
			// Both ffmpeg input and whisper use files in the temp dir.
			for _, a := range args {
				if strings.Contains(a, "kapso-whisper-") {
					capturedDir = filepath.Dir(a)
					break
				}
			}
		}
		if name == "ffmpeg" {
			// Write a dummy WAV file.
			if len(args) > 0 {
				wavPath := args[len(args)-1]
				_ = os.WriteFile(wavPath, []byte("dummy-wav"), 0o600)
			}
			return exec.CommandContext(ctx, "true")
		}
		// whisper-cli: write transcript file.
		prefix := findArgAfter(args, "-of")
		if prefix != "" {
			_ = os.WriteFile(prefix+".txt", []byte("hello"), 0o600)
		}
		return exec.CommandContext(ctx, "true")
	}

	_, err = p.Transcribe(context.Background(), []byte("audio"), "audio/ogg")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	if capturedDir == "" {
		t.Fatal("could not capture temp dir from execCmd args")
	}

	// Temp dir should be gone after Transcribe returns.
	if _, statErr := os.Stat(capturedDir); !os.IsNotExist(statErr) {
		t.Errorf("temp dir %q still exists after Transcribe", capturedDir)
	}
}

func TestLocalWhisperTempCleanupOnError(t *testing.T) {
	// Verify cleanup even when ffmpeg fails.
	cfg := config.TranscribeConfig{
		BinaryPath: "whisper-cli",
		ModelPath:  "/models/ggml-base.bin",
	}
	p, err := newLocalWhisper(cfg)
	if err != nil {
		t.Fatalf("newLocalWhisper: %v", err)
	}

	var capturedDir string

	p.execCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if capturedDir == "" {
			for _, a := range args {
				if strings.Contains(a, "kapso-whisper-") {
					capturedDir = filepath.Dir(a)
					break
				}
			}
		}
		// Always fail ffmpeg.
		if name == "ffmpeg" {
			return exec.CommandContext(ctx, "false")
		}
		return exec.CommandContext(ctx, "true")
	}

	_, err = p.Transcribe(context.Background(), []byte("audio"), "audio/ogg")
	if err == nil {
		t.Fatal("expected error from ffmpeg failure")
	}

	if capturedDir == "" {
		// If ffmpeg fails before we can capture, skip check.
		t.Log("could not capture temp dir (ffmpeg may have failed before first arg captured)")
		return
	}

	if _, statErr := os.Stat(capturedDir); !os.IsNotExist(statErr) {
		t.Errorf("temp dir %q still exists after failed Transcribe", capturedDir)
	}
}

// Ensure localWhisper implements Transcriber at compile time.
var _ Transcriber = (*localWhisper)(nil)

// Ensure unused import is referenced.
var _ = fmt.Sprintf
