package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTranscribeDefaults verifies that defaults() returns the expected
// zero values and non-zero defaults for TranscribeConfig.
func TestTranscribeDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.Transcribe.MaxAudioSize != 25*1024*1024 {
		t.Errorf("MaxAudioSize default: got %d, want %d", cfg.Transcribe.MaxAudioSize, 25*1024*1024)
	}
	if cfg.Transcribe.BinaryPath != "whisper-cli" {
		t.Errorf("BinaryPath default: got %q, want %q", cfg.Transcribe.BinaryPath, "whisper-cli")
	}
	if cfg.Transcribe.Timeout != 30 {
		t.Errorf("Timeout default: got %d, want %d", cfg.Transcribe.Timeout, 30)
	}
	if cfg.Transcribe.Provider != "" {
		t.Errorf("Provider default: got %q, want empty string", cfg.Transcribe.Provider)
	}
	if cfg.Transcribe.APIKey != "" {
		t.Errorf("APIKey default: got %q, want empty string", cfg.Transcribe.APIKey)
	}
	if cfg.Transcribe.Model != "" {
		t.Errorf("Model default: got %q, want empty string", cfg.Transcribe.Model)
	}
	if cfg.Transcribe.Language != "" {
		t.Errorf("Language default: got %q, want empty string", cfg.Transcribe.Language)
	}
	if cfg.Transcribe.ModelPath != "" {
		t.Errorf("ModelPath default: got %q, want empty string", cfg.Transcribe.ModelPath)
	}
}

// TestTranscribeEnvOverrides verifies that each KAPSO_TRANSCRIBE_* env var
// is applied correctly by applyEnv().
func TestTranscribeEnvOverrides(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		check   func(cfg *Config) bool
		wantMsg string
	}{
		{
			name:    "KAPSO_TRANSCRIBE_PROVIDER lowercased",
			envKey:  "KAPSO_TRANSCRIBE_PROVIDER",
			envVal:  "Groq",
			check:   func(cfg *Config) bool { return cfg.Transcribe.Provider == "groq" },
			wantMsg: `Provider should be "groq" (lowercased)`,
		},
		{
			name:    "KAPSO_TRANSCRIBE_API_KEY",
			envKey:  "KAPSO_TRANSCRIBE_API_KEY",
			envVal:  "sk-testkey",
			check:   func(cfg *Config) bool { return cfg.Transcribe.APIKey == "sk-testkey" },
			wantMsg: `APIKey should be "sk-testkey"`,
		},
		{
			name:    "KAPSO_TRANSCRIBE_MODEL",
			envKey:  "KAPSO_TRANSCRIBE_MODEL",
			envVal:  "whisper-large-v3",
			check:   func(cfg *Config) bool { return cfg.Transcribe.Model == "whisper-large-v3" },
			wantMsg: `Model should be "whisper-large-v3"`,
		},
		{
			name:    "KAPSO_TRANSCRIBE_LANGUAGE",
			envKey:  "KAPSO_TRANSCRIBE_LANGUAGE",
			envVal:  "en",
			check:   func(cfg *Config) bool { return cfg.Transcribe.Language == "en" },
			wantMsg: `Language should be "en"`,
		},
		{
			name:    "KAPSO_TRANSCRIBE_MAX_AUDIO_SIZE parsed as int64",
			envKey:  "KAPSO_TRANSCRIBE_MAX_AUDIO_SIZE",
			envVal:  "52428800",
			check:   func(cfg *Config) bool { return cfg.Transcribe.MaxAudioSize == 52428800 },
			wantMsg: "MaxAudioSize should be 52428800",
		},
		{
			name:    "KAPSO_TRANSCRIBE_BINARY_PATH",
			envKey:  "KAPSO_TRANSCRIBE_BINARY_PATH",
			envVal:  "/usr/local/bin/whisper",
			check:   func(cfg *Config) bool { return cfg.Transcribe.BinaryPath == "/usr/local/bin/whisper" },
			wantMsg: `BinaryPath should be "/usr/local/bin/whisper"`,
		},
		{
			name:    "KAPSO_TRANSCRIBE_MODEL_PATH",
			envKey:  "KAPSO_TRANSCRIBE_MODEL_PATH",
			envVal:  "/models/ggml-large.bin",
			check:   func(cfg *Config) bool { return cfg.Transcribe.ModelPath == "/models/ggml-large.bin" },
			wantMsg: `ModelPath should be "/models/ggml-large.bin"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			cfg := defaults()
			applyEnv(&cfg)
			if !tt.check(&cfg) {
				t.Error(tt.wantMsg)
			}
		})
	}
}

// TestTranscribeTOMLParsing verifies that a [transcribe] TOML section is
// decoded correctly into Config.Transcribe.
func TestTranscribeTOMLParsing(t *testing.T) {
	tomlContent := `
[transcribe]
provider = "openai"
api_key = "toml-key"
model = "whisper-1"
language = "fr"
max_audio_size = 10485760
binary_path = "/opt/whisper"
model_path = "/opt/models/ggml-base.bin"
timeout = 60
`
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgFile, []byte(tomlContent), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	t.Setenv("KAPSO_CONFIG", cfgFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Transcribe.Provider != "openai" {
		t.Errorf("Provider: got %q, want %q", cfg.Transcribe.Provider, "openai")
	}
	if cfg.Transcribe.APIKey != "toml-key" {
		t.Errorf("APIKey: got %q, want %q", cfg.Transcribe.APIKey, "toml-key")
	}
	if cfg.Transcribe.Model != "whisper-1" {
		t.Errorf("Model: got %q, want %q", cfg.Transcribe.Model, "whisper-1")
	}
	if cfg.Transcribe.Language != "fr" {
		t.Errorf("Language: got %q, want %q", cfg.Transcribe.Language, "fr")
	}
	if cfg.Transcribe.MaxAudioSize != 10485760 {
		t.Errorf("MaxAudioSize: got %d, want %d", cfg.Transcribe.MaxAudioSize, 10485760)
	}
	if cfg.Transcribe.BinaryPath != "/opt/whisper" {
		t.Errorf("BinaryPath: got %q, want %q", cfg.Transcribe.BinaryPath, "/opt/whisper")
	}
	if cfg.Transcribe.ModelPath != "/opt/models/ggml-base.bin" {
		t.Errorf("ModelPath: got %q, want %q", cfg.Transcribe.ModelPath, "/opt/models/ggml-base.bin")
	}
	if cfg.Transcribe.Timeout != 60 {
		t.Errorf("Timeout: got %d, want %d", cfg.Transcribe.Timeout, 60)
	}
}

// TestTranscribePrecedence verifies the 3-tier precedence:
// env var > TOML file value > default.
func TestTranscribePrecedence(t *testing.T) {
	tomlContent := `
[transcribe]
provider = "deepgram"
model = "nova-2"
`
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgFile, []byte(tomlContent), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	t.Setenv("KAPSO_CONFIG", cfgFile)
	// Env var overrides the TOML "deepgram" value.
	t.Setenv("KAPSO_TRANSCRIBE_PROVIDER", "GROQ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Env var should win (lowercased).
	if cfg.Transcribe.Provider != "groq" {
		t.Errorf("Provider precedence: got %q, want %q (env should override TOML)", cfg.Transcribe.Provider, "groq")
	}
	// TOML should win over default for Model.
	if cfg.Transcribe.Model != "nova-2" {
		t.Errorf("Model precedence: got %q, want %q (TOML should override default)", cfg.Transcribe.Model, "nova-2")
	}
	// Default should apply for unset fields.
	if cfg.Transcribe.BinaryPath != "whisper-cli" {
		t.Errorf("BinaryPath precedence: got %q, want %q (default should apply)", cfg.Transcribe.BinaryPath, "whisper-cli")
	}
}

// TestTranscribeValidateZeroMaxAudioSize verifies that Validate() resets
// MaxAudioSize to 25MB when it is zero or negative (TOML zero-value masking guard).
func TestTranscribeValidateZeroMaxAudioSize(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  int64
	}{
		{"zero value", 0, 25 * 1024 * 1024},
		{"negative value", -1, 25 * 1024 * 1024},
		{"positive value unchanged", 10 * 1024 * 1024, 10 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaults()
			cfg.Transcribe.MaxAudioSize = tt.input
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
			if cfg.Transcribe.MaxAudioSize != tt.want {
				t.Errorf("MaxAudioSize after Validate: got %d, want %d", cfg.Transcribe.MaxAudioSize, tt.want)
			}
		})
	}
}

// TestGatewayDefaults verifies that defaults() returns the expected role and scopes.
func TestGatewayDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.Gateway.Role != "operator" {
		t.Errorf("Role default: got %q, want %q", cfg.Gateway.Role, "operator")
	}
	if len(cfg.Gateway.Scopes) != 2 || cfg.Gateway.Scopes[0] != "operator.read" || cfg.Gateway.Scopes[1] != "operator.write" {
		t.Errorf("Scopes default: got %v, want [operator.read operator.write]", cfg.Gateway.Scopes)
	}
}

// TestGatewayEnvOverrides verifies GATEWAY_ROLE and GATEWAY_SCOPES env vars.
func TestGatewayEnvOverrides(t *testing.T) {
	t.Run("GATEWAY_ROLE", func(t *testing.T) {
		t.Setenv("GATEWAY_ROLE", "viewer")
		cfg := defaults()
		applyEnv(&cfg)
		if cfg.Gateway.Role != "viewer" {
			t.Errorf("Role: got %q, want %q", cfg.Gateway.Role, "viewer")
		}
	})

	t.Run("GATEWAY_SCOPES", func(t *testing.T) {
		t.Setenv("GATEWAY_SCOPES", "operator.read")
		cfg := defaults()
		applyEnv(&cfg)
		if len(cfg.Gateway.Scopes) != 1 || cfg.Gateway.Scopes[0] != "operator.read" {
			t.Errorf("Scopes: got %v, want [operator.read]", cfg.Gateway.Scopes)
		}
	})

	t.Run("GATEWAY_SCOPES trims whitespace", func(t *testing.T) {
		t.Setenv("GATEWAY_SCOPES", "operator.read, operator.write")
		cfg := defaults()
		applyEnv(&cfg)
		if len(cfg.Gateway.Scopes) != 2 || cfg.Gateway.Scopes[1] != "operator.write" {
			t.Errorf("Scopes should trim whitespace: got %v", cfg.Gateway.Scopes)
		}
	})
}

// TestGatewayValidateEmptyRoleScopes verifies Validate() resets empty role/scopes.
func TestGatewayValidateEmptyRoleScopes(t *testing.T) {
	cfg := defaults()
	cfg.Gateway.Role = ""
	cfg.Gateway.Scopes = nil
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if cfg.Gateway.Role != "operator" {
		t.Errorf("Role after Validate: got %q, want %q", cfg.Gateway.Role, "operator")
	}
	if len(cfg.Gateway.Scopes) != 2 {
		t.Errorf("Scopes after Validate: got %v, want [operator.read operator.write]", cfg.Gateway.Scopes)
	}
}

// TestTranscribeEmptyProviderNoError verifies that Load() succeeds when no
// transcribe provider is configured (empty provider is valid).
func TestTranscribeEmptyProviderNoError(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	// Empty config file — no [transcribe] section.
	if err := os.WriteFile(cfgFile, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	t.Setenv("KAPSO_CONFIG", cfgFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with empty provider: unexpected error: %v", err)
	}
	if cfg.Transcribe.Provider != "" {
		t.Errorf("Provider should be empty, got %q", cfg.Transcribe.Provider)
	}
}
