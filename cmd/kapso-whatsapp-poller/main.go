package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/config"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/delivery"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/delivery/poller"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/delivery/webhook"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/relay"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/tailscale"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	if cfg.Kapso.APIKey == "" || cfg.Kapso.PhoneNumberID == "" {
		log.Fatal("KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
	}

	mode := cfg.Delivery.Mode
	if (mode == "tailscale" || mode == "domain") && cfg.Webhook.VerifyToken == "" {
		log.Fatal("KAPSO_WEBHOOK_VERIFY_TOKEN must be set when using tailscale or domain mode")
	}

	// Connect to OpenClaw gateway.
	gw := gateway.NewClient(cfg.Gateway.URL, cfg.Gateway.Token)
	if err := gw.Connect(); err != nil {
		log.Fatalf("failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	client := kapso.NewClient(cfg.Kapso.APIKey, cfg.Kapso.PhoneNumberID)

	// Graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Build source(s) based on mode.
	var sources []delivery.Source
	var funnelProc *os.Process

	runPolling := mode == "polling" || cfg.Delivery.PollFallback

	if runPolling {
		sources = append(sources, &poller.Poller{
			Client:    client,
			Interval:  time.Duration(cfg.Delivery.PollInterval) * time.Second,
			StateDir:  cfg.State.Dir,
			StateFile: filepath.Join(cfg.State.Dir, "last-poll"),
		})
		log.Printf("polling every %ds, gateway=%s session=%s",
			cfg.Delivery.PollInterval, cfg.Gateway.URL, cfg.Gateway.SessionKey)
	}

	if mode == "tailscale" || mode == "domain" {
		sources = append(sources, &webhook.Server{
			Addr:        cfg.Webhook.Addr,
			VerifyToken: cfg.Webhook.VerifyToken,
			AppSecret:   cfg.Webhook.Secret,
			Client:      client,
		})

		if mode == "tailscale" {
			_, port, err := net.SplitHostPort(cfg.Webhook.Addr)
			if err != nil {
				port = strings.TrimPrefix(cfg.Webhook.Addr, ":")
			}
			webhookURL, proc, err := tailscale.StartFunnel(port)
			if err != nil {
				log.Fatalf("tailscale funnel: %v", err)
			}
			funnelProc = proc
			log.Printf("register this webhook URL in Kapso: %s", webhookURL)
		}

		if mode == "domain" {
			log.Printf("webhook server listening, point your reverse proxy at %s", cfg.Webhook.Addr)
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
		SessionsJSON: cfg.Gateway.SessionsJSON,
		SessionKey:   cfg.Gateway.SessionKey,
		Client:       client,
		Tracker:      relay.NewTracker(),
	}

	// Consume loop — identical for all sources.
	go func() {
		for evt := range events {
			if err := gw.Send(cfg.Gateway.SessionKey, evt.ID, evt.Text); err != nil {
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
