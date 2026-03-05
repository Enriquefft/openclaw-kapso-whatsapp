// Package commands implements the bridge-level command system.
// Commands are intercepted before the gateway and executed directly by the bridge,
// so they work even when the agent context is stale or the gateway is restarting.
package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/kapso"
)

const maxOutputLen = 4000

// Dispatcher parses and executes bridge commands.
type Dispatcher struct {
	prefix  string
	timeout time.Duration
	defs    map[string]config.CommandDef
}

// New creates a Dispatcher from config. Returns a no-op dispatcher if no
// commands are configured (IsCommand always returns false).
func New(cfg config.CommandsConfig) *Dispatcher {
	return &Dispatcher{
		prefix:  cfg.Prefix,
		timeout: time.Duration(cfg.Timeout) * time.Second,
		defs:    cfg.Definitions,
	}
}

// Prefix returns the configured command prefix string.
func (d *Dispatcher) Prefix() string { return d.prefix }

// IsCommand reports whether text is a command invocation (starts with prefix).
// Always returns false when no prefix is configured or no commands are defined.
func (d *Dispatcher) IsCommand(text string) bool {
	if d.prefix == "" || len(d.defs) == 0 {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(text), d.prefix)
}

// Parse extracts the command name and free-form args from text.
// Returns ok=false if the text doesn't start with the prefix.
func (d *Dispatcher) Parse(text string) (name, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, d.prefix) {
		return "", "", false
	}
	rest := strings.TrimSpace(text[len(d.prefix):])
	parts := strings.SplitN(rest, " ", 2)
	name = strings.ToLower(parts[0])
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return name, args, true
}

// Exists reports whether a command with the given name is defined (or is the built-in "help").
func (d *Dispatcher) Exists(name string) bool {
	if name == "help" {
		return true
	}
	_, ok := d.defs[name]
	return ok
}

// CanRun reports whether the role is permitted to run the named command.
// Returns false for unknown commands. Empty Roles list means all roles allowed.
func (d *Dispatcher) CanRun(name, role string) bool {
	if name == "help" {
		return true
	}
	def, ok := d.defs[name]
	if !ok {
		return false
	}
	if len(def.Roles) == 0 {
		return true
	}
	for _, r := range def.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Ack returns the pre-execution acknowledgment message for a command, or "".
func (d *Dispatcher) Ack(name string) string {
	def, ok := d.defs[name]
	if !ok {
		return ""
	}
	return def.Ack
}

// Handle executes the named command and returns the reply text.
// For shell commands: runs the shell string with timeout, returns combined output.
// For agent commands: sends the prompt template (with {args} injected) to the gateway.
// For the built-in "help": returns a formatted list of commands the role can use.
func (d *Dispatcher) Handle(
	ctx context.Context,
	name, args, role, sessionKey string,
	gw gateway.Gateway,
	req *gateway.Request,
	_ *kapso.Client,
) string {
	if name == "help" {
		return d.helpText(role)
	}

	def, ok := d.defs[name]
	if !ok {
		return fmt.Sprintf("Unknown command. Send %shelp for available commands.", d.prefix)
	}

	switch def.Type {
	case "shell":
		return d.runShell(ctx, def, args)
	case "agent":
		return d.runAgent(ctx, def, args, role, sessionKey, gw, req)
	default:
		log.Printf("commands: unknown command type %q for %q", def.Type, name)
		return fmt.Sprintf("Command %q has an invalid type %q.", name, def.Type)
	}
}

// runShell executes a shell command and returns its combined output.
func (d *Dispatcher) runShell(ctx context.Context, def config.CommandDef, args string) string {
	timeout := d.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shellStr := strings.ReplaceAll(def.Shell, "{args}", args)
	cmd := exec.CommandContext(tCtx, "sh", "-c", shellStr)
	cmd.Env = append(os.Environ(), "ARGS="+args)

	out, err := cmd.CombinedOutput()

	if tCtx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("Command timed out after %s.", timeout)
	}

	result := strings.TrimSpace(string(out))
	if result == "" && err != nil {
		return fmt.Sprintf("Command failed: %v", err)
	}
	if len(result) > maxOutputLen {
		result = result[:maxOutputLen] + "\n… (truncated)"
	}
	return result
}

// runAgent sends a templated prompt to the gateway and returns the agent's reply.
func (d *Dispatcher) runAgent(
	ctx context.Context,
	def config.CommandDef,
	args, role, sessionKey string,
	gw gateway.Gateway,
	req *gateway.Request,
) string {
	prompt := strings.ReplaceAll(def.Prompt, "{args}", args)
	reply, err := gw.SendAndReceive(ctx, &gateway.Request{
		SessionKey:     sessionKey,
		IdempotencyKey: req.IdempotencyKey + "-cmd",
		From:           req.From,
		FromName:       req.FromName,
		Role:           role,
		Text:           prompt,
	})
	if err != nil {
		log.Printf("commands: agent command failed: %v", err)
		return fmt.Sprintf("Agent error: %v", err)
	}
	return reply
}

// helpText returns a formatted list of commands accessible to the given role.
func (d *Dispatcher) helpText(role string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("*Available commands* (prefix: %s)", d.prefix))

	// Collect and sort command names for stable output.
	var names []string
	for name := range d.defs {
		if d.CanRun(name, role) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		def := d.defs[name]
		desc := def.Description
		if desc == "" {
			desc = def.Type + " command"
		}
		lines = append(lines, fmt.Sprintf("%s%s — %s", d.prefix, name, desc))
	}

	if len(names) == 0 {
		return "No commands available for your role."
	}

	lines = append(lines, fmt.Sprintf("\nSend %s<command> [optional instructions]", d.prefix))
	return strings.Join(lines, "\n")
}
