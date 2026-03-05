package commands

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/gateway"
)

// mockGateway records what was sent and returns a canned reply.
type mockGateway struct {
	lastReq *gateway.Request
	reply   string
	err     error
}

func (m *mockGateway) Connect(_ context.Context) error { return nil }
func (m *mockGateway) Close() error                    { return nil }
func (m *mockGateway) SendAndReceive(_ context.Context, req *gateway.Request) (string, error) {
	m.lastReq = req
	return m.reply, m.err
}

// newDispatcher builds a Dispatcher with common test defaults.
func newDispatcher(prefix string, defs map[string]config.CommandDef) *Dispatcher {
	return New(config.CommandsConfig{
		Prefix:      prefix,
		Timeout:     5,
		Definitions: defs,
	})
}

// ── IsCommand ────────────────────────────────────────────────────────────────

func TestIsCommandDormantWhenUnconfigured(t *testing.T) {
	// No prefix, no defs — the system must be a true no-op.
	d := New(config.CommandsConfig{})
	cases := []string{"!help", "hello", "!cmd foo"}
	for _, text := range cases {
		if d.IsCommand(text) {
			t.Errorf("IsCommand(%q) = true on unconfigured dispatcher, want false", text)
		}
	}
}

func TestIsCommandDormantWithDefsButNoPrefix(t *testing.T) {
	d := New(config.CommandsConfig{
		Definitions: map[string]config.CommandDef{"ping": {Type: "shell", Shell: "echo pong"}},
	})
	if d.IsCommand("ping") {
		t.Error("IsCommand should be false when prefix is empty")
	}
}

func TestIsCommandMatchesPrefix(t *testing.T) {
	d := newDispatcher("!", map[string]config.CommandDef{
		"ping": {Type: "shell", Shell: "echo pong"},
	})
	if !d.IsCommand("!ping") {
		t.Error("expected IsCommand true for !ping")
	}
	if d.IsCommand("ping") {
		t.Error("expected IsCommand false without prefix")
	}
	if !d.IsCommand("  !ping") {
		// IsCommand trims leading whitespace before checking the prefix.
		t.Error("leading whitespace should be tolerated")
	}
}

// ── Parse ────────────────────────────────────────────────────────────────────

func TestParse(t *testing.T) {
	d := newDispatcher("!", map[string]config.CommandDef{
		"cmd": {Type: "shell", Shell: "echo $ARGS"},
	})

	cases := []struct {
		input    string
		wantName string
		wantArgs string
		wantOK   bool
	}{
		{"!cmd", "cmd", "", true},
		{"!cmd  hello world", "cmd", "hello world", true},
		{"!CMD upper", "cmd", "upper", true}, // name lowercased
		{"  !cmd trimmed  ", "cmd", "trimmed", true},
		{"cmd no prefix", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range cases {
		name, args, ok := d.Parse(tt.input)
		if ok != tt.wantOK || name != tt.wantName || args != tt.wantArgs {
			t.Errorf("Parse(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, name, args, ok, tt.wantName, tt.wantArgs, tt.wantOK)
		}
	}
}

// ── CanRun / role enforcement ─────────────────────────────────────────────────

func TestCanRunRoleEnforcement(t *testing.T) {
	d := newDispatcher("!", map[string]config.CommandDef{
		"admin-only": {Type: "shell", Shell: "id", Roles: []string{"admin"}},
		"all":        {Type: "shell", Shell: "echo hi"},
	})

	if d.CanRun("admin-only", "member") {
		t.Error("member should not be able to run admin-only")
	}
	if !d.CanRun("admin-only", "admin") {
		t.Error("admin should be able to run admin-only")
	}
	if !d.CanRun("all", "member") {
		t.Error("empty roles list means all roles allowed")
	}
	if !d.CanRun("help", "member") {
		t.Error("built-in help must always be allowed")
	}
	if d.CanRun("nonexistent", "admin") {
		t.Error("unknown command should not be runnable")
	}
}

// ── Shell execution ───────────────────────────────────────────────────────────

func TestShellCommandOutput(t *testing.T) {
	d := newDispatcher("!", map[string]config.CommandDef{
		"greet": {Type: "shell", Shell: "echo hello"},
	})

	reply := d.Handle(context.Background(), "greet", "", "admin", "s", nil, &gateway.Request{}, nil)
	if reply != "hello" {
		t.Errorf("expected 'hello', got %q", reply)
	}
}

func TestShellCommandArgsViaEnv(t *testing.T) {
	// $ARGS env var must carry the user-supplied arguments.
	d := newDispatcher("!", map[string]config.CommandDef{
		"echo": {Type: "shell", Shell: "echo $ARGS"},
	})

	reply := d.Handle(context.Background(), "echo", "world", "admin", "s", nil, &gateway.Request{}, nil)
	if reply != "world" {
		t.Errorf("expected 'world' via $ARGS, got %q", reply)
	}
}

func TestShellTemplateArgsNotInterpolated(t *testing.T) {
	// {args} in shell templates must NOT be substituted with user input.
	// If it were, a semicolon in args would chain additional shell commands.
	// User input flows only through $ARGS env var.
	d := newDispatcher("!", map[string]config.CommandDef{
		// Template naively contains {args} — must stay literal after our fix.
		"cmd": {Type: "shell", Shell: "echo {args}"},
	})

	// If {args} were substituted: shell runs "echo data; echo INJECTED"
	// → two lines of output, second being "INJECTED".
	// Without substitution: shell runs "echo {args}" → prints "{args}".
	reply := d.Handle(context.Background(), "cmd", "data; echo INJECTED", "admin", "s", nil, &gateway.Request{}, nil)

	if strings.Contains(reply, "INJECTED") {
		t.Errorf("{args} was interpolated into the shell template — injection succeeded: %q", reply)
	}
	if reply != "{args}" {
		t.Errorf("expected shell to output literal '{args}', got %q", reply)
	}
}

func TestShellCommandTimeout(t *testing.T) {
	d := New(config.CommandsConfig{
		Prefix:  "!",
		Timeout: 1, // 1 second
		Definitions: map[string]config.CommandDef{
			"slow": {Type: "shell", Shell: "sleep 10"},
		},
	})

	start := time.Now()
	reply := d.Handle(context.Background(), "slow", "", "admin", "s", nil, &gateway.Request{}, nil)
	elapsed := time.Since(start)

	if !strings.Contains(reply, "timed out") {
		t.Errorf("expected timeout message, got %q", reply)
	}
	if elapsed > 3*time.Second {
		t.Errorf("command took too long (%s); timeout not enforced", elapsed)
	}
}

func TestShellOutputTruncation(t *testing.T) {
	// Generate output larger than maxOutputLen (4000 bytes) using portable sh.
	d := newDispatcher("!", map[string]config.CommandDef{
		"big": {Type: "shell", Shell: "head -c 5001 /dev/zero | tr '\\0' 'x'"},
	})

	reply := d.Handle(context.Background(), "big", "", "admin", "s", nil, &gateway.Request{}, nil)
	if len(reply) > maxOutputLen+50 { // small buffer for the truncation suffix
		t.Errorf("output not truncated: len=%d", len(reply))
	}
	if !strings.Contains(reply, "truncated") {
		t.Errorf("expected truncation indicator, got %q", reply[:min(100, len(reply))])
	}
}

// ── Agent command ─────────────────────────────────────────────────────────────

func TestAgentCommandSendsTemplatedPrompt(t *testing.T) {
	gw := &mockGateway{reply: "agent says hi"}
	d := newDispatcher("!", map[string]config.CommandDef{
		"ask": {Type: "agent", Prompt: "Translate to French: {args}"},
	})

	req := &gateway.Request{IdempotencyKey: "msg1", From: "+1234", Role: "member"}
	reply := d.Handle(context.Background(), "ask", "hello", "member", "sess", gw, req, nil)

	if reply != "agent says hi" {
		t.Errorf("unexpected reply: %q", reply)
	}
	if gw.lastReq == nil {
		t.Fatal("gateway was not called")
	}
	if gw.lastReq.Text != "Translate to French: hello" {
		t.Errorf("prompt not templated correctly: %q", gw.lastReq.Text)
	}
}

func TestAgentCommandPropagatesError(t *testing.T) {
	gw := &mockGateway{err: errors.New("gateway down")}
	d := newDispatcher("!", map[string]config.CommandDef{
		"ask": {Type: "agent", Prompt: "Do: {args}"},
	})

	req := &gateway.Request{}
	reply := d.Handle(context.Background(), "ask", "anything", "admin", "s", gw, req, nil)
	if !strings.Contains(reply, "Agent error") {
		t.Errorf("expected agent error message, got %q", reply)
	}
}

// ── Built-in help ─────────────────────────────────────────────────────────────

func TestHelpFiltersbyRole(t *testing.T) {
	d := newDispatcher("!", map[string]config.CommandDef{
		"public": {Type: "shell", Shell: "echo", Description: "public cmd"},
		"secret": {Type: "shell", Shell: "id", Description: "admin only", Roles: []string{"admin"}},
	})

	memberHelp := d.Handle(context.Background(), "help", "", "member", "s", nil, &gateway.Request{}, nil)
	if strings.Contains(memberHelp, "secret") {
		t.Error("member help should not expose admin-only command")
	}
	if !strings.Contains(memberHelp, "public") {
		t.Error("member help should include public command")
	}

	adminHelp := d.Handle(context.Background(), "help", "", "admin", "s", nil, &gateway.Request{}, nil)
	if !strings.Contains(adminHelp, "secret") {
		t.Error("admin help should include admin-only command")
	}
}

func TestHelpNoCommandsForRole(t *testing.T) {
	d := newDispatcher("!", map[string]config.CommandDef{
		"admin-only": {Type: "shell", Shell: "id", Roles: []string{"admin"}},
	})

	reply := d.Handle(context.Background(), "help", "", "guest", "s", nil, &gateway.Request{}, nil)
	if !strings.Contains(reply, "No commands available") {
		t.Errorf("expected no-commands message for role with no access, got %q", reply)
	}
}

// min is available from Go 1.21 builtins but added here for clarity.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
