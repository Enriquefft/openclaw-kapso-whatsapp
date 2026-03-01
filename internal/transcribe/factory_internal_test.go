package transcribe

import (
	"testing"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
)

// TestNewWrapsCloudProvidersWithRetry verifies that all cloud providers (openai,
// groq, deepgram) are wrapped in a *retryTranscriber by the factory.
func TestNewWrapsCloudProvidersWithRetry(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.TranscribeConfig
		wantNil  bool
		wantErr  bool
	}{
		{
			name: "openai wrapped in retryTranscriber",
			cfg:  config.TranscribeConfig{Provider: "openai", APIKey: "sk-test"},
		},
		{
			name: "groq wrapped in retryTranscriber",
			cfg:  config.TranscribeConfig{Provider: "groq", APIKey: "gsk-test"},
		},
		{
			name: "deepgram wrapped in retryTranscriber",
			cfg:  config.TranscribeConfig{Provider: "deepgram", APIKey: "dg-test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := New(tc.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil transcriber")
			}
			// Verify the returned value is a *retryTranscriber.
			if _, ok := got.(*retryTranscriber); !ok {
				t.Errorf("factory returned %T, want *retryTranscriber", got)
			}
		})
	}
}
