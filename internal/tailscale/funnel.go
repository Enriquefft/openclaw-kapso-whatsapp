package tailscale

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// tsStatus is a minimal subset of `tailscale status --json` output.
type tsStatus struct {
	Self struct {
		DNSName string `json:"DNSName"`
	} `json:"Self"`
}

// EnsureInstalled checks that the tailscale CLI is available.
func EnsureInstalled() error {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return fmt.Errorf("tailscale CLI not found in PATH — install from https://tailscale.com/download")
	}
	return nil
}

// PublicURL returns the deterministic HTTPS URL for a funnelled port,
// e.g. "https://machine.tailnet.ts.net".
func PublicURL() (string, error) {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return "", fmt.Errorf("tailscale status: %w (is tailscale running?)", err)
	}

	var status tsStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return "", fmt.Errorf("parse tailscale status: %w", err)
	}

	dns := strings.TrimSuffix(status.Self.DNSName, ".")
	if dns == "" {
		return "", fmt.Errorf("tailscale: empty DNS name — is the node connected?")
	}

	return "https://" + dns, nil
}

// StartFunnel runs `tailscale funnel <port>` in the background.
// It returns the public webhook URL (https://<machine>.<tailnet>.ts.net/webhook).
// The caller owns the process and must kill it on shutdown.
func StartFunnel(port string) (webhookURL string, proc *os.Process, err error) {
	if err := EnsureInstalled(); err != nil {
		return "", nil, err
	}

	baseURL, err := PublicURL()
	if err != nil {
		return "", nil, err
	}

	// Start `tailscale funnel <port>` in the background.
	cmd := exec.Command("tailscale", "funnel", port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start tailscale funnel: %w", err)
	}

	webhookURL = baseURL + "/webhook"
	log.Printf("tailscale funnel started on port %s → %s", port, webhookURL)

	return webhookURL, cmd.Process, nil
}
