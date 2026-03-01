package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
)

// localWhisper implements Transcriber using local ffmpeg + whisper-cli subprocesses.
// Audio bytes are converted from OGG to WAV via ffmpeg, then transcribed by whisper-cli.
// All temp files are cleaned up after each call, including on context cancellation.
type localWhisper struct {
	// BinaryPath is the path to the whisper-cli binary (default: "whisper-cli").
	BinaryPath string

	// ModelPath is the path to the GGML model file (required).
	ModelPath string

	// Language is an optional language hint passed to whisper-cli via -l.
	Language string

	// execCmd is injectable for testing. Defaults to exec.CommandContext.
	execCmd func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// newLocalWhisper constructs a localWhisper from the given config.
// Returns an error if ModelPath is empty.
func newLocalWhisper(cfg config.TranscribeConfig) (*localWhisper, error) {
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("local provider requires model_path (set KAPSO_TRANSCRIBE_MODEL_PATH)")
	}

	binaryPath := cfg.BinaryPath
	if binaryPath == "" {
		binaryPath = "whisper-cli"
	}

	return &localWhisper{
		BinaryPath: binaryPath,
		ModelPath:  cfg.ModelPath,
		Language:   cfg.Language,
		execCmd:    exec.CommandContext,
	}, nil
}

// Transcribe converts audio bytes to transcript text using ffmpeg and whisper-cli.
//
// Flow:
//  1. Write audio bytes to a temp dir as audio.ogg
//  2. Convert OGG to WAV via ffmpeg (16kHz mono PCM)
//  3. Run whisper-cli to produce a .txt transcript
//  4. Read and return the trimmed transcript text
//
// The temp directory is always removed after the call, including on error.
func (p *localWhisper) Transcribe(ctx context.Context, audio []byte, _ string) (string, error) {
	// Create a temp directory for all intermediate files.
	dir, err := os.MkdirTemp("", "kapso-whisper-*")
	if err != nil {
		return "", fmt.Errorf("local provider: create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Write raw audio bytes.
	rawPath := filepath.Join(dir, "audio.ogg")
	if err := os.WriteFile(rawPath, audio, 0o600); err != nil {
		return "", fmt.Errorf("local provider: write audio file: %w", err)
	}

	// Convert OGG → 16kHz mono WAV via ffmpeg.
	wavPath := filepath.Join(dir, "audio.wav")
	ffmpegCmd := p.execCmd(ctx, "ffmpeg",
		"-y",
		"-loglevel", "error",
		"-i", rawPath,
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", "16000",
		wavPath,
	)
	if out, err := ffmpegCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Build whisper-cli args.
	outputPrefix := filepath.Join(dir, "transcript")
	args := []string{
		"-m", p.ModelPath,
		"-f", wavPath,
		"-otxt",
		"-of", outputPrefix,
	}
	if p.Language != "" {
		args = append(args, "-l", p.Language)
	}

	// Run whisper-cli to produce outputPrefix + ".txt".
	whisperCmd := p.execCmd(ctx, p.BinaryPath, args...)
	if out, err := whisperCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper-cli transcription failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Read the generated transcript file.
	txtPath := outputPrefix + ".txt"
	raw, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("local provider: read transcript file: %w", err)
	}

	return strings.TrimSpace(string(raw)), nil
}
