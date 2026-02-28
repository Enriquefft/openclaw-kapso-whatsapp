package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for the kapso-whatsapp bridge.
type Config struct {
	Kapso    KapsoConfig    `toml:"kapso"`
	Delivery DeliveryConfig `toml:"delivery"`
	Webhook  WebhookConfig  `toml:"webhook"`
	Gateway  GatewayConfig  `toml:"gateway"`
	State    StateConfig    `toml:"state"`
}

type KapsoConfig struct {
	APIKey        string `toml:"api_key"`
	PhoneNumberID string `toml:"phone_number_id"`
}

type DeliveryConfig struct {
	Mode         string `toml:"mode"`
	PollInterval int    `toml:"poll_interval"`
	PollFallback bool   `toml:"poll_fallback"`
}

type WebhookConfig struct {
	Addr        string `toml:"addr"`
	VerifyToken string `toml:"verify_token"`
	Secret      string `toml:"secret"`
}

type GatewayConfig struct {
	URL          string `toml:"url"`
	Token        string `toml:"token"`
	SessionKey   string `toml:"session_key"`
	SessionsJSON string `toml:"sessions_json"`
}

type StateConfig struct {
	Dir string `toml:"dir"`
}

func defaults() Config {
	home := os.Getenv("HOME")
	return Config{
		Delivery: DeliveryConfig{
			Mode:         "polling",
			PollInterval: 30,
		},
		Webhook: WebhookConfig{
			Addr: ":18790",
		},
		Gateway: GatewayConfig{
			URL:          "ws://127.0.0.1:18789",
			SessionKey:   "main",
			SessionsJSON: filepath.Join(home, ".openclaw", "agents", "main", "sessions", "sessions.json"),
		},
		State: StateConfig{
			Dir: filepath.Join(home, ".config", "kapso-whatsapp"),
		},
	}
}

// Load reads configuration from the TOML config file (if it exists) and
// applies environment variable overrides. Env vars always win.
//
// Config file resolution: KAPSO_CONFIG env var → ~/.config/kapso-whatsapp/config.toml → skip.
func Load() (*Config, error) {
	cfg := defaults()

	path := configPath()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, &cfg); err != nil {
				return nil, err
			}
		}
	}

	applyEnv(&cfg)
	return &cfg, nil
}

func configPath() string {
	if p := os.Getenv("KAPSO_CONFIG"); p != "" {
		return expandHome(p)
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "kapso-whatsapp", "config.toml")
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("KAPSO_API_KEY"); v != "" {
		cfg.Kapso.APIKey = v
	}
	if v := os.Getenv("KAPSO_PHONE_NUMBER_ID"); v != "" {
		cfg.Kapso.PhoneNumberID = v
	}

	if v := os.Getenv("KAPSO_MODE"); v != "" {
		cfg.Delivery.Mode = resolveMode(v, "")
	} else if v := os.Getenv("KAPSO_WEBHOOK_MODE"); v != "" {
		cfg.Delivery.Mode = resolveMode("", v)
	}
	if v := os.Getenv("KAPSO_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Delivery.PollInterval = n
		}
	}
	if v := os.Getenv("KAPSO_POLL_FALLBACK"); v != "" {
		cfg.Delivery.PollFallback = v == "true"
	}

	if v := os.Getenv("KAPSO_WEBHOOK_ADDR"); v != "" {
		cfg.Webhook.Addr = v
	}
	if v := os.Getenv("KAPSO_WEBHOOK_VERIFY_TOKEN"); v != "" {
		cfg.Webhook.VerifyToken = v
	}
	if v := os.Getenv("KAPSO_WEBHOOK_SECRET"); v != "" {
		cfg.Webhook.Secret = v
	}

	if v := os.Getenv("OPENCLAW_GATEWAY_URL"); v != "" {
		cfg.Gateway.URL = v
	}
	if v := os.Getenv("OPENCLAW_TOKEN"); v != "" {
		cfg.Gateway.Token = v
	}
	if v := os.Getenv("OPENCLAW_SESSION_KEY"); v != "" {
		cfg.Gateway.SessionKey = v
	}
	if v := os.Getenv("OPENCLAW_SESSIONS_JSON"); v != "" {
		cfg.Gateway.SessionsJSON = v
	}

	if v := os.Getenv("KAPSO_STATE_DIR"); v != "" {
		cfg.State.Dir = v
	}
}

// resolveMode normalises the delivery mode from KAPSO_MODE (preferred) or
// the deprecated KAPSO_WEBHOOK_MODE.
func resolveMode(mode, legacyMode string) string {
	switch strings.ToLower(mode) {
	case "polling", "tailscale", "domain":
		return strings.ToLower(mode)
	}

	switch strings.ToLower(legacyMode) {
	case "webhook", "both":
		return "domain"
	}

	return "polling"
}

// Validate checks that required fields are set for the configured mode.
func (c *Config) Validate() error {
	if c.Delivery.PollInterval < 5 {
		c.Delivery.PollInterval = 30
	}

	mode := strings.ToLower(c.Delivery.Mode)
	switch mode {
	case "polling", "tailscale", "domain":
		c.Delivery.Mode = mode
	default:
		c.Delivery.Mode = "polling"
	}

	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
