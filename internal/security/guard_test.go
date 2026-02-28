package security

import (
	"testing"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/config"
)

func testCfg() config.SecurityConfig {
	return config.SecurityConfig{
		Mode: "allowlist",
		Roles: map[string][]string{
			"admin":  {"+1234567890"},
			"member": {"+0987654321", "+1122334455"},
		},
		DenyMessage:      "denied",
		RateLimit:        3,
		RateWindow:       60,
		SessionIsolation: true,
		DefaultRole:      "member",
	}
}

func TestAllowlistAllow(t *testing.T) {
	g := New(testCfg())
	if v := g.Check("+1234567890"); v != Allow {
		t.Fatalf("expected Allow, got %d", v)
	}
}

func TestAllowlistDeny(t *testing.T) {
	g := New(testCfg())
	if v := g.Check("+9999999999"); v != Deny {
		t.Fatalf("expected Deny, got %d", v)
	}
}

func TestOpenModeAllowsAnyone(t *testing.T) {
	cfg := testCfg()
	cfg.Mode = "open"
	g := New(cfg)
	if v := g.Check("+9999999999"); v != Allow {
		t.Fatalf("expected Allow in open mode, got %d", v)
	}
}

func TestRoleResolution(t *testing.T) {
	g := New(testCfg())

	if r := g.Role("+1234567890"); r != "admin" {
		t.Fatalf("expected admin, got %s", r)
	}
	if r := g.Role("+0987654321"); r != "member" {
		t.Fatalf("expected member, got %s", r)
	}
}

func TestRoleDefaultInOpenMode(t *testing.T) {
	cfg := testCfg()
	cfg.Mode = "open"
	g := New(cfg)

	if r := g.Role("+9999999999"); r != "member" {
		t.Fatalf("expected default role member, got %s", r)
	}
}

func TestRateLimiting(t *testing.T) {
	cfg := testCfg()
	cfg.RateLimit = 2
	g := New(cfg)

	if v := g.Check("+1234567890"); v != Allow {
		t.Fatalf("first check: expected Allow, got %d", v)
	}
	if v := g.Check("+1234567890"); v != Allow {
		t.Fatalf("second check: expected Allow, got %d", v)
	}
	if v := g.Check("+1234567890"); v != RateLimited {
		t.Fatalf("third check: expected RateLimited, got %d", v)
	}
}

func TestRateLimitWindowReset(t *testing.T) {
	cfg := testCfg()
	cfg.RateLimit = 1
	cfg.RateWindow = 60
	g := New(cfg)

	now := time.Now()
	g.now = func() time.Time { return now }

	if v := g.Check("+1234567890"); v != Allow {
		t.Fatalf("expected Allow, got %d", v)
	}
	if v := g.Check("+1234567890"); v != RateLimited {
		t.Fatalf("expected RateLimited, got %d", v)
	}

	// Advance past window.
	g.now = func() time.Time { return now.Add(61 * time.Second) }
	if v := g.Check("+1234567890"); v != Allow {
		t.Fatalf("expected Allow after window reset, got %d", v)
	}
}

func TestSessionKeyIsolation(t *testing.T) {
	g := New(testCfg())
	key := g.SessionKey("main", "+1234567890")
	if key != "main-wa-1234567890" {
		t.Fatalf("expected main-wa-1234567890, got %s", key)
	}
}

func TestSessionKeyNoIsolation(t *testing.T) {
	cfg := testCfg()
	cfg.SessionIsolation = false
	g := New(cfg)
	key := g.SessionKey("main", "+1234567890")
	if key != "main" {
		t.Fatalf("expected main, got %s", key)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"+1 (234) 567-890", "1234567890"},
		{"1234567890", "1234567890"},
		{"+1234567890", "1234567890"},
		{"51926689401", "51926689401"},
		{"+51926689401", "51926689401"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalize(tt.input)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizedPhoneLookup(t *testing.T) {
	cfg := testCfg()
	cfg.Roles = map[string][]string{
		"admin": {"+1 (234) 567-890"},
	}
	g := New(cfg)

	// Should match after normalization.
	if v := g.Check("+1234567890"); v != Allow {
		t.Fatalf("expected Allow with normalized phone, got %d", v)
	}
	if r := g.Role("+1234567890"); r != "admin" {
		t.Fatalf("expected admin role, got %s", r)
	}
}

func TestDenyMessage(t *testing.T) {
	g := New(testCfg())
	if g.DenyMessage() != "denied" {
		t.Fatalf("expected 'denied', got %q", g.DenyMessage())
	}
}
