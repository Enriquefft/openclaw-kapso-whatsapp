package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// tsStatus is a minimal subset of `tailscale status --json` output.
type tsStatus struct {
	Self struct {
		DNSName string `json:"DNSName"`
	} `json:"Self"`
}

// FunnelError classifies tailscale failures as fatal or retryable.
type FunnelError struct {
	Msg       string
	Retryable bool
}

func (e *FunnelError) Error() string { return e.Msg }

// EnsureInstalled checks that the tailscale CLI is available.
func EnsureInstalled() error {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return fmt.Errorf("tailscale CLI not found in PATH — install from https://tailscale.com/download")
	}
	return nil
}

// publicURL queries tailscale status and returns the HTTPS base URL.
// statusFunc allows injection for testing; nil uses the real CLI.
func publicURL(statusFunc func() ([]byte, error)) (string, error) {
	if statusFunc == nil {
		statusFunc = func() ([]byte, error) {
			return exec.Command("tailscale", "status", "--json").CombinedOutput()
		}
	}

	out, err := statusFunc()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.ToLower(string(exitErr.Stderr))
			combined := strings.ToLower(string(out))
			if strings.Contains(stderr, "access denied") || strings.Contains(stderr, "not an operator") ||
				strings.Contains(combined, "access denied") || strings.Contains(combined, "not an operator") {
				return "", &FunnelError{
					Msg:       "tailscale funnel requires --operator=$USER or root. See: https://tailscale.com/kb/1312/serve#permissions",
					Retryable: false,
				}
			}
		}
		return "", &FunnelError{
			Msg:       fmt.Sprintf("tailscale status: %v (is tailscale running?)", err),
			Retryable: true,
		}
	}

	var status tsStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return "", &FunnelError{
			Msg:       fmt.Sprintf("parse tailscale status: %v", err),
			Retryable: true,
		}
	}

	dns := strings.TrimSuffix(status.Self.DNSName, ".")
	if dns == "" {
		return "", &FunnelError{
			Msg:       "tailscale: empty DNS name — is the node connected?",
			Retryable: true,
		}
	}

	return "https://" + dns, nil
}

// PublicURL returns the deterministic HTTPS URL for a funnelled port,
// e.g. "https://machine.tailnet.ts.net".
func PublicURL() (string, error) {
	return publicURL(nil)
}

// FunnelConfig holds retry and dependency-injection options for StartFunnelWithRetry.
// Zero values use sensible defaults (1s base, 60s max, 2x factor).
type FunnelConfig struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Factor     float64
	SleepFunc  func(time.Duration)
	StatusFunc func() ([]byte, error)
	StartFunc  func(port string) (*exec.Cmd, error)
}

func (c *FunnelConfig) defaults() {
	if c.BaseDelay == 0 {
		c.BaseDelay = 1 * time.Second
	}
	if c.MaxDelay == 0 {
		c.MaxDelay = 60 * time.Second
	}
	if c.Factor == 0 {
		c.Factor = 2.0
	}
	if c.SleepFunc == nil {
		c.SleepFunc = time.Sleep
	}
	if c.StatusFunc == nil {
		c.StatusFunc = func() ([]byte, error) {
			return exec.Command("tailscale", "status", "--json").CombinedOutput()
		}
	}
	if c.StartFunc == nil {
		c.StartFunc = func(port string) (*exec.Cmd, error) {
			cmd := exec.Command("tailscale", "funnel", port)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd, cmd.Start()
		}
	}
}

// StartFunnelWithRetry runs `tailscale funnel <port>` with exponential backoff
// for retryable errors (node not connected, daemon not running).
// Fatal errors (CLI missing, no permissions) return immediately.
func StartFunnelWithRetry(ctx context.Context, port string, cfg FunnelConfig) (webhookURL string, proc *os.Process, err error) {
	cfg.defaults()

	if err := EnsureInstalled(); err != nil {
		return "", nil, err
	}

	delay := cfg.BaseDelay
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, fmt.Errorf("tailscale setup cancelled: %w", err)
		}

		baseURL, urlErr := publicURL(cfg.StatusFunc)
		if urlErr == nil {
			cmd, startErr := cfg.StartFunc(port)
			if startErr != nil {
				return "", nil, fmt.Errorf("start tailscale funnel: %w (check that funnel is enabled — see `tailscale funnel --help`)", startErr)
			}

			webhookURL = baseURL + "/webhook"
			log.Printf("tailscale funnel started on port %s → %s", port, webhookURL)
			return webhookURL, cmd.Process, nil
		}

		var fe *FunnelError
		if errors.As(urlErr, &fe) && !fe.Retryable {
			return "", nil, urlErr
		}

		log.Printf("tailscale not ready, retrying in %s: %v", delay, urlErr)
		cfg.SleepFunc(delay)

		if err := ctx.Err(); err != nil {
			return "", nil, fmt.Errorf("tailscale setup cancelled: %w", err)
		}

		delay = min(time.Duration(float64(delay)*cfg.Factor), cfg.MaxDelay)
	}
}

// StartFunnel runs `tailscale funnel <port>` in the background without retry.
// Deprecated: prefer StartFunnelWithRetry for resilient startup.
func StartFunnel(port string) (webhookURL string, proc *os.Process, err error) {
	return StartFunnelWithRetry(context.Background(), port, FunnelConfig{})
}
