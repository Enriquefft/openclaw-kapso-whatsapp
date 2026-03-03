package security

import (
	"testing"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
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
		{"15551234567", "15551234567"},
		{"+15551234567", "15551234567"},
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

func testGroupCfg() config.SecurityConfig {
	cfg := testCfg()
	cfg.GroupPrefix = "!claw"
	cfg.GroupIDs = []string{"120363001@g.us", "120363002@g.us"}
	return cfg
}

func TestIsGroup(t *testing.T) {
	g := New(testCfg())
	if !g.IsGroup("120363001@g.us") {
		t.Fatal("expected @g.us suffix to be detected as group")
	}
	if g.IsGroup("+15551234567") {
		t.Fatal("expected phone number to not be detected as group")
	}
	if g.IsGroup("") {
		t.Fatal("expected empty string to not be detected as group")
	}
}

func TestCheckGroupAllowedWithPrefix(t *testing.T) {
	g := New(testGroupCfg())
	v := g.CheckGroup("+1234567890", "120363001@g.us", "!claw what is the weather?")
	if v != Allow {
		t.Fatalf("expected Allow, got %d", v)
	}
	stripped := g.StripPrefix("!claw what is the weather?")
	if stripped != "what is the weather?" {
		t.Fatalf("expected stripped text, got %q", stripped)
	}
}

func TestCheckGroupDenyUnauthorized(t *testing.T) {
	g := New(testGroupCfg())
	v := g.CheckGroup("+9999999999", "120363001@g.us", "!claw hello")
	if v != Deny {
		t.Fatalf("expected Deny for unknown sender, got %d", v)
	}
}

func TestCheckGroupSkipNoPrefix(t *testing.T) {
	g := New(testGroupCfg())
	v := g.CheckGroup("+1234567890", "120363001@g.us", "just a normal message")
	if v != Skip {
		t.Fatalf("expected Skip when prefix missing, got %d", v)
	}
}

func TestCheckGroupDenyUnknownGroup(t *testing.T) {
	g := New(testGroupCfg())
	v := g.CheckGroup("+1234567890", "999999999@g.us", "!claw hello")
	if v != Deny {
		t.Fatalf("expected Deny for unknown group, got %d", v)
	}
}

func TestCheckGroupOpenModeNoPrefix(t *testing.T) {
	cfg := testGroupCfg()
	cfg.Mode = "open"
	cfg.GroupPrefix = ""
	cfg.GroupIDs = nil
	g := New(cfg)

	v := g.CheckGroup("+9999999999", "120363999@g.us", "hello everyone")
	if v != Allow {
		t.Fatalf("expected Allow in open mode with no prefix, got %d", v)
	}
}

func TestStripPrefix(t *testing.T) {
	g := New(testGroupCfg())

	tests := []struct {
		input, want string
	}{
		{"!claw what is this?", "what is this?"},
		{"  !claw  spaced  ", "spaced"},
		{"no prefix here", "no prefix here"},
		{"!claw", ""},
	}
	for _, tt := range tests {
		got := g.StripPrefix(tt.input)
		if got != tt.want {
			t.Errorf("StripPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
