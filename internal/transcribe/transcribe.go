package transcribe

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
)

// Transcriber converts audio bytes to text.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
}

// New constructs a Transcriber from the provided config.
//
// Returns (nil, nil) when no provider is configured — transcription disabled.
// Returns an error when the provider is known but misconfigured or not yet implemented.
// Returns an error for unknown providers.
func New(cfg config.TranscribeConfig) (Transcriber, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))

	if provider == "" {
		log.Printf("transcription disabled (no provider configured)")
		return nil, nil
	}

	switch provider {
	case "openai", "groq", "deepgram":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider %q requires KAPSO_TRANSCRIBE_API_KEY", provider)
		}
		return nil, fmt.Errorf("provider %q not yet implemented (Phase 2)", provider)

	case "local":
		binaryPath := cfg.BinaryPath
		if binaryPath == "" {
			binaryPath = "whisper-cli"
		}
		if _, err := exec.LookPath(binaryPath); err != nil {
			return nil, fmt.Errorf("local provider binary %q not found: %w", binaryPath, err)
		}
		return nil, fmt.Errorf("local provider not yet implemented (Phase 3)")

	default:
		return nil, fmt.Errorf("unknown transcription provider %q (valid: openai, groq, deepgram, local)", provider)
	}
}
