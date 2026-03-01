package transcribe_test

import (
	"strings"
	"testing"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/transcribe"
)


func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.TranscribeConfig
		wantNil     bool // true if we expect (nil, nil)
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty provider returns nil nil",
			cfg:     config.TranscribeConfig{Provider: ""},
			wantNil: true,
		},
		{
			name: "openai with api key returns non-nil transcriber",
			cfg:  config.TranscribeConfig{Provider: "openai", APIKey: "sk-test"},
		},
		{
			name: "groq with api key returns non-nil transcriber",
			cfg:  config.TranscribeConfig{Provider: "groq", APIKey: "gsk-test"},
		},
		{
			name: "deepgram with api key returns non-nil transcriber",
			cfg:  config.TranscribeConfig{Provider: "deepgram", APIKey: "dg-test"},
		},
		{
			name:        "openai without api key returns key error",
			cfg:         config.TranscribeConfig{Provider: "openai", APIKey: ""},
			wantErr:     true,
			errContains: "requires KAPSO_TRANSCRIBE_API_KEY",
		},
		{
			name:        "groq without api key returns key error",
			cfg:         config.TranscribeConfig{Provider: "groq", APIKey: ""},
			wantErr:     true,
			errContains: "requires KAPSO_TRANSCRIBE_API_KEY",
		},
		{
			name:        "deepgram without api key returns key error",
			cfg:         config.TranscribeConfig{Provider: "deepgram", APIKey: ""},
			wantErr:     true,
			errContains: "requires KAPSO_TRANSCRIBE_API_KEY",
		},
		{
			name:        "unknown provider returns error",
			cfg:         config.TranscribeConfig{Provider: "unknown"},
			wantErr:     true,
			errContains: "unknown transcription provider",
		},
		{
			name: "uppercase OPENAI normalizes to openai behavior",
			cfg:  config.TranscribeConfig{Provider: "OPENAI", APIKey: "sk-test"},
			// Now returns a working provider, not an error.
		},
		{
			name: "whitespace openai normalizes to openai behavior",
			cfg:  config.TranscribeConfig{Provider: " openai ", APIKey: "sk-test"},
			// Now returns a working provider, not an error.
		},
		{
			name:        "local with non-existent binary returns error about binary not found",
			cfg:         config.TranscribeConfig{Provider: "local", BinaryPath: "/nonexistent/whisper-cli"},
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := transcribe.New(tc.cfg)

			if tc.wantNil {
				if got != nil {
					t.Errorf("expected nil transcriber, got %v", got)
				}
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
				return
			}

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				return
			}

			// If neither wantNil nor wantErr, we expect (non-nil, nil).
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil transcriber")
			}
		})
	}
}
