package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/delivery"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/delivery/poller"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/delivery/webhook"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/relay"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/tailscale"
)

func main() {
	apiKey := os.Getenv("KAPSO_API_KEY")
	phoneNumberID := os.Getenv("KAPSO_PHONE_NUMBER_ID")
	gatewayURL := envOr("OPENCLAW_GATEWAY_URL", "ws://127.0.0.1:18789")
	gatewayToken := os.Getenv("OPENCLAW_TOKEN")
	sessionKey := envOr("OPENCLAW_SESSION_KEY", "main")
	intervalStr := envOr("KAPSO_POLL_INTERVAL", "30")
	stateDir := envOr("KAPSO_STATE_DIR", filepath.Join(os.Getenv("HOME"), ".config", "kapso-whatsapp"))
	sessionsJSON := envOr("OPENCLAW_SESSIONS_JSON",
		filepath.Join(os.Getenv("HOME"), ".openclaw", "agents", "main", "sessions", "sessions.json"))

	// Delivery mode: "polling" (default), "tailscale", "domain".
	mode := resolveMode(envOr("KAPSO_MODE", ""), envOr("KAPSO_WEBHOOK_MODE", ""))
	pollFallback := envOr("KAPSO_POLL_FALLBACK", "false") == "true"

	// Webhook configuration (used by tailscale and domain modes).
	webhookAddr := envOr("KAPSO_WEBHOOK_ADDR", ":18790")
	webhookVerifyToken := os.Getenv("KAPSO_WEBHOOK_VERIFY_TOKEN")
	webhookSecret := os.Getenv("KAPSO_WEBHOOK_SECRET")

	if apiKey == "" || phoneNumberID == "" {
		log.Fatal("KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
	}

	if mode == "tailscale" || mode == "domain" {
		if webhookVerifyToken == "" {
			log.Fatal("KAPSO_WEBHOOK_VERIFY_TOKEN must be set when using tailscale or domain mode")
		}
	}

	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 5 {
		interval = 30
	}

	// Connect to OpenClaw gateway.
	gw := gateway.NewClient(gatewayURL, gatewayToken)
	if err := gw.Connect(); err != nil {
		log.Fatalf("failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	client := kapso.NewClient(apiKey, phoneNumberID)

	// Graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Build source(s) based on mode.
	var sources []delivery.Source
	var funnelProc *os.Process

	runPolling := mode == "polling" || pollFallback

	if runPolling {
		sources = append(sources, &poller.Poller{
			Client:    client,
			Interval:  time.Duration(interval) * time.Second,
			StateDir:  stateDir,
			StateFile: filepath.Join(stateDir, "last-poll"),
		})
		log.Printf("polling every %ds, gateway=%s session=%s", interval, gatewayURL, sessionKey)
	}

	if mode == "tailscale" || mode == "domain" {
		sources = append(sources, &webhook.Server{
			Addr:        webhookAddr,
			VerifyToken: webhookVerifyToken,
			AppSecret:   webhookSecret,
			Client:      client,
		})

		if mode == "tailscale" {
			_, port, err := net.SplitHostPort(webhookAddr)
			if err != nil {
				port = strings.TrimPrefix(webhookAddr, ":")
			}
			webhookURL, proc, err := tailscale.StartFunnel(port)
			if err != nil {
				log.Fatalf("tailscale funnel: %v", err)
			}
			funnelProc = proc
			log.Printf("register this webhook URL in Kapso: %s", webhookURL)
		}

		if mode == "domain" {
			log.Printf("webhook server listening, point your reverse proxy at %s", webhookAddr)
		}
	}

	if !runPolling && mode != "tailscale" && mode != "domain" {
		log.Fatal("no delivery source configured")
	}

	// Fan-in + dedup.
	merge := &delivery.Merge{Sources: sources}
	events := make(chan delivery.Event, 64)

	go merge.Run(ctx, events)
	go merge.StartCleanup(ctx, 10*time.Minute)

	// Relay agent replies back to WhatsApp.
	rel := &relay.Relay{
		SessionsJSON: sessionsJSON,
		SessionKey:   sessionKey,
		Client:       client,
		Tracker:      relay.NewTracker(),
	}

	// Consume loop — identical for all sources.
	go func() {
		for evt := range events {
			if err := gw.Send(sessionKey, evt.ID, evt.Text); err != nil {
				log.Printf("error forwarding message %s: %v", evt.ID, err)
				continue
			}
			log.Printf("forwarded message %s from %s", evt.ID, evt.From)
			go rel.Send(ctx, evt.From, time.Now().UTC())
		}
	}()

	// Block until shutdown signal.
	sig := <-stop
	log.Printf("received %s, shutting down", sig)
	cancel()
	cleanupFunnel(funnelProc)
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
		log.Printf("KAPSO_WEBHOOK_MODE is deprecated — use KAPSO_MODE=domain or KAPSO_MODE=tailscale instead")
		return "domain"
	}

	return "polling"
}

// cleanupFunnel gracefully stops the tailscale funnel process if it was started.
func cleanupFunnel(proc *os.Process) {
	if proc == nil {
		return
	}
	log.Printf("stopping tailscale funnel (pid %d)", proc.Pid)
	proc.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("tailscale funnel did not exit, sending SIGKILL")
		proc.Kill()
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
