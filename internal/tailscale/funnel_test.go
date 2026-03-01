package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func statusJSON(dns string) []byte {
	s := tsStatus{}
	s.Self.DNSName = dns
	b, _ := json.Marshal(s)
	return b
}

func TestStartFunnelWithRetry(t *testing.T) {
	tests := []struct {
		name        string
		statusCalls []struct {
			out []byte
			err error
		}
		startErr    error
		wantURL     string
		wantErr     string
		wantRetries int // expected number of sleep calls
	}{
		{
			name: "immediate success",
			statusCalls: []struct {
				out []byte
				err error
			}{
				{out: statusJSON("machine.tailnet.ts.net.")},
			},
			wantURL:     "https://machine.tailnet.ts.net/webhook",
			wantRetries: 0,
		},
		{
			name: "empty DNS then success",
			statusCalls: []struct {
				out []byte
				err error
			}{
				{out: statusJSON("")},
				{out: statusJSON("machine.tailnet.ts.net.")},
			},
			wantURL:     "https://machine.tailnet.ts.net/webhook",
			wantRetries: 1,
		},
		{
			name: "permission denied is fatal",
			statusCalls: []struct {
				out []byte
				err error
			}{
				{
					out: []byte("access denied: not an operator"),
					err: &exec.ExitError{},
				},
			},
			wantErr:     "requires --operator=$USER or root",
			wantRetries: 0,
		},
		{
			name: "start funnel fails",
			statusCalls: []struct {
				out []byte
				err error
			}{
				{out: statusJSON("machine.tailnet.ts.net.")},
			},
			startErr:    errors.New("funnel not enabled"),
			wantErr:     "start tailscale funnel",
			wantRetries: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callIdx := 0
			var sleeps []time.Duration

			cfg := FunnelConfig{
				BaseDelay: 100 * time.Millisecond,
				MaxDelay:  10 * time.Second,
				Factor:    2.0,
				SleepFunc: func(d time.Duration) { sleeps = append(sleeps, d) },
				StatusFunc: func() ([]byte, error) {
					if callIdx >= len(tt.statusCalls) {
						t.Fatal("unexpected extra status call")
					}
					c := tt.statusCalls[callIdx]
					callIdx++
					return c.out, c.err
				},
				StartFunc: func(port string) (*exec.Cmd, error) {
					if tt.startErr != nil {
						return nil, tt.startErr
					}
					// Return a dummy cmd with a non-nil Process for testing.
					cmd := &exec.Cmd{}
					return cmd, nil
				},
			}

			url, _, err := StartFunnelWithRetry(context.Background(), "8080", cfg)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if got := err.Error(); !contains(got, tt.wantErr) {
					t.Fatalf("error %q does not contain %q", got, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if url != tt.wantURL {
				t.Errorf("url = %q, want %q", url, tt.wantURL)
			}
			if len(sleeps) != tt.wantRetries {
				t.Errorf("retries = %d, want %d", len(sleeps), tt.wantRetries)
			}
		})
	}
}

func TestStartFunnelWithRetry_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	cfg := FunnelConfig{
		BaseDelay: 100 * time.Millisecond,
		MaxDelay:  10 * time.Second,
		Factor:    2.0,
		SleepFunc: func(d time.Duration) {
			cancel() // cancel after first sleep
		},
		StatusFunc: func() ([]byte, error) {
			calls++
			return statusJSON(""), nil // always empty DNS
		},
		StartFunc: func(port string) (*exec.Cmd, error) {
			t.Fatal("start should not be called")
			return nil, nil
		},
	}

	_, _, err := StartFunnelWithRetry(ctx, "8080", cfg)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !contains(err.Error(), "cancelled") {
		t.Errorf("error %q should mention cancellation", err)
	}
}

func TestStartFunnelWithRetry_ExponentialBackoff(t *testing.T) {
	var sleeps []time.Duration
	callIdx := 0
	// 4 failures then success
	statuses := []struct {
		out []byte
		err error
	}{
		{out: statusJSON("")},
		{out: statusJSON("")},
		{out: statusJSON("")},
		{out: statusJSON("")},
		{out: statusJSON("node.ts.net.")},
	}

	cfg := FunnelConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  10 * time.Second,
		Factor:    2.0,
		SleepFunc: func(d time.Duration) { sleeps = append(sleeps, d) },
		StatusFunc: func() ([]byte, error) {
			c := statuses[callIdx]
			callIdx++
			return c.out, c.err
		},
		StartFunc: func(port string) (*exec.Cmd, error) {
			return &exec.Cmd{}, nil
		},
	}

	_, _, err := StartFunnelWithRetry(context.Background(), "8080", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sleeps) != 4 {
		t.Fatalf("expected 4 sleeps, got %d", len(sleeps))
	}

	// Verify exponential growth: 1s, 2s, 4s, 8s
	expected := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for i, want := range expected {
		if sleeps[i] != want {
			t.Errorf("sleep[%d] = %v, want %v", i, sleeps[i], want)
		}
	}
}

func TestStartFunnelWithRetry_BackoffCap(t *testing.T) {
	var sleeps []time.Duration
	callIdx := 0
	// 5 failures then success — delay should cap at MaxDelay
	statuses := make([]struct {
		out []byte
		err error
	}, 6)
	for i := range 5 {
		statuses[i] = struct {
			out []byte
			err error
		}{out: statusJSON("")}
	}
	statuses[5] = struct {
		out []byte
		err error
	}{out: statusJSON("node.ts.net.")}

	cfg := FunnelConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  3 * time.Second,
		Factor:    2.0,
		SleepFunc: func(d time.Duration) { sleeps = append(sleeps, d) },
		StatusFunc: func() ([]byte, error) {
			c := statuses[callIdx]
			callIdx++
			return c.out, c.err
		},
		StartFunc: func(port string) (*exec.Cmd, error) {
			return &exec.Cmd{}, nil
		},
	}

	_, _, err := StartFunnelWithRetry(context.Background(), "8080", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1s, 2s, 3s (capped), 3s (capped), 3s (capped)
	expected := []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second, 3 * time.Second}
	if len(sleeps) != len(expected) {
		t.Fatalf("expected %d sleeps, got %d", len(expected), len(sleeps))
	}
	for i, want := range expected {
		if sleeps[i] != want {
			t.Errorf("sleep[%d] = %v, want %v", i, sleeps[i], want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
