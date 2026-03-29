package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/commands"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/delivery"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/delivery/poller"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/delivery/webhook"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/device"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/kapso"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/security"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/tailscale"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/transcribe"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Build transcriber from config (nil when disabled, fatal on misconfiguration).
	transcriber, err := transcribe.New(cfg.Transcribe)
	if err != nil {
		log.Fatalf("transcription config error: %v", err)
	}
	if cfg.Kapso.APIKey == "" || cfg.Kapso.PhoneNumberID == "" {
		log.Fatal("KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
	}

	mode := cfg.Delivery.Mode
	if (mode == "tailscale" || mode == "domain") && cfg.Webhook.VerifyToken == "" {
		log.Fatal("KAPSO_WEBHOOK_VERIFY_TOKEN must be set when using tailscale or domain mode")
	}

	// Graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load or create device identity for gateway authentication.
	ident, err := device.LoadOrCreate(cfg.State.Dir)
	if err != nil {
		log.Fatalf("device identity: %v", err)
	}
	log.Printf("device: id=%s", ident.DeviceID()[:16])

	// Connect to the AI gateway (OpenClaw, ZeroClaw, etc.).
	gw, err := gateway.New(cfg.Gateway, gateway.WithSigner(ident))
	if err != nil {
		log.Fatalf("invalid gateway config: %v", err)
	}
	if err := gw.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to gateway: %v", err)
	}
	defer func() { _ = gw.Close() }()

	gwType := cfg.Gateway.Type
	if gwType == "" {
		gwType = "openclaw"
	}
	log.Printf("gateway: type=%s url=%s", gwType, cfg.Gateway.URL)

	client := kapso.NewClient(cfg.Kapso.APIKey, cfg.Kapso.PhoneNumberID)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Build source(s) based on mode.
	var sources []delivery.Source
	var funnelProc *os.Process

	runPolling := mode == "polling" || cfg.Delivery.PollFallback

	if runPolling {
		sources = append(sources, &poller.Poller{
			Client:       client,
			Interval:     time.Duration(cfg.Delivery.PollInterval) * time.Second,
			StateDir:     cfg.State.Dir,
			StateFile:    filepath.Join(cfg.State.Dir, "last-poll"),
			Transcriber:  transcriber,
			MaxAudioSize: cfg.Transcribe.MaxAudioSize,
		})
		log.Printf("polling every %ds, gateway=%s session=%s",
			cfg.Delivery.PollInterval, cfg.Gateway.URL, cfg.Gateway.SessionKey)
	}

	if mode == "tailscale" || mode == "domain" {
		sources = append(sources, &webhook.Server{
			Addr:         cfg.Webhook.Addr,
			VerifyToken:  cfg.Webhook.VerifyToken,
			AppSecret:    cfg.Webhook.Secret,
			Client:       client,
			Transcriber:  transcriber,
			MaxAudioSize: cfg.Transcribe.MaxAudioSize,
		})

		if mode == "tailscale" {
			_, port, err := net.SplitHostPort(cfg.Webhook.Addr)
			if err != nil {
				port = strings.TrimPrefix(cfg.Webhook.Addr, ":")
			}
			webhookURL, proc, err := tailscale.StartFunnelWithRetry(ctx, port, tailscale.FunnelConfig{})
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

	go func() { _ = merge.Run(ctx, events) }()
	go merge.StartCleanup(ctx, 10*time.Minute)

	// Security guard.
	guard := security.New(cfg.Security)
	log.Printf("security: mode=%s, session_isolation=%v, rate_limit=%d/%ds",
		cfg.Security.Mode, cfg.Security.SessionIsolation,
		cfg.Security.RateLimit, cfg.Security.RateWindow)

	// Command dispatcher (no-op when no commands are configured).
	dispatcher := commands.New(cfg.Commands)
	if cfg.Commands.Prefix != "" && len(cfg.Commands.Definitions) > 0 {
		log.Printf("commands: prefix=%q, %d command(s) configured", cfg.Commands.Prefix, len(cfg.Commands.Definitions))
	}

	// Consume loop — identical for all sources.
	go func() {
		for evt := range events {
			verdict := guard.Check(evt.From)
			switch verdict {
			case security.Deny:
				log.Printf("guard: blocked unauthorized sender %s", evt.From)
				if msg := guard.DenyMessage(); msg != "" {
					if _, err := client.SendText(evt.From, msg); err != nil {
						log.Printf("guard: failed to send deny message to %s: %v", evt.From, err)
					}
				}
				continue
			case security.RateLimited:
				log.Printf("guard: rate limited sender %s", evt.From)
				continue
			}

			role := guard.Role(evt.From)
			sessionKey := guard.SessionKey(cfg.Gateway.SessionKey, evt.From)

			// Bridge commands are intercepted before the gateway.
			if dispatcher.IsCommand(evt.Text) {
				go handleCommand(ctx, dispatcher, gw, client, evt, sessionKey, role)
				continue
			}

			// Forward to gateway and wait for agent reply in a goroutine.
			go handleMessage(ctx, gw, client, evt, sessionKey, role, cfg.Gateway.ErrorMessage)
		}
	}()

	// Block until shutdown signal.
	sig := <-stop
	log.Printf("received %s, shutting down", sig)
	cancel()
	cleanupFunnel(funnelProc)
}

// handleCommand dispatches a bridge-level command and sends the reply to WhatsApp.
// Commands are executed without involving the AI gateway (except agent-type commands).
func handleCommand(ctx context.Context, d *commands.Dispatcher, gw gateway.Gateway, client *kapso.Client, evt delivery.Event, sessionKey, role string) {
	from := evt.From
	if !strings.HasPrefix(from, "+") {
		from = "+" + from
	}

	if err := client.MarkReadWithTyping(evt.ID); err != nil {
		log.Printf("command: failed to mark read for %s: %v", evt.ID, err)
	}

	name, args, ok := d.Parse(evt.Text)
	if !ok {
		msg := fmt.Sprintf("Unknown command. Send %shelp for available commands.", d.Prefix())
		if _, err := client.SendText(from, msg); err != nil {
			log.Printf("command: failed to send reply to %s: %v", from, err)
		}
		_ = client.MarkRead(evt.ID)
		return
	}
	if !d.Exists(name) {
		msg := fmt.Sprintf("Unknown command %s%s. Send %shelp for available commands.", d.Prefix(), name, d.Prefix())
		if _, err := client.SendText(from, msg); err != nil {
			log.Printf("command: failed to send reply to %s: %v", from, err)
		}
		_ = client.MarkRead(evt.ID)
		return
	}
	if !d.CanRun(name, role) {
		msg := fmt.Sprintf("You don't have permission to use %s%s.", d.Prefix(), name)
		if _, err := client.SendText(from, msg); err != nil {
			log.Printf("command: failed to send reply to %s: %v", from, err)
		}
		_ = client.MarkRead(evt.ID)
		return
	}

	log.Printf("command: %q args=%q from=%s role=%s", name, args, evt.From, role)

	// Send ack before potentially slow or self-terminating commands.
	if ack := d.Ack(name); ack != "" {
		if _, err := client.SendText(from, ack); err != nil {
			log.Printf("command: failed to send ack to %s: %v", from, err)
		}
	}

	req := &gateway.Request{
		SessionKey:     sessionKey,
		IdempotencyKey: evt.ID,
		From:           evt.From,
		FromName:       evt.Name,
		Role:           role,
	}
	reply := d.Handle(ctx, name, args, role, sessionKey, gw, req, client)
	if reply != "" {
		chunks := gateway.SplitMessage(gateway.MdToWhatsApp(reply), 4096)
		for _, chunk := range chunks {
			if _, err := client.SendText(from, chunk); err != nil {
				log.Printf("command: failed to send reply chunk to %s: %v", from, err)
			}
		}
	}

	if err := client.MarkRead(evt.ID); err != nil {
		log.Printf("command: failed to dismiss typing for %s: %v", evt.ID, err)
	}
}

// handleMessage sends a message to the gateway, waits for the agent's reply,
// and sends it back to the WhatsApp sender.
func handleMessage(ctx context.Context, gw gateway.Gateway, client *kapso.Client, evt delivery.Event, sessionKey, role, errorMessage string) {
	from := evt.From
	if !strings.HasPrefix(from, "+") {
		from = "+" + from
	}

	// Show typing indicator.
	if err := client.MarkReadWithTyping(evt.ID); err != nil {
		log.Printf("relay: failed to mark read with typing for %s: %v", evt.ID, err)
	}

	// Refresh typing periodically while waiting.
	typingCtx, typingCancel := context.WithCancel(ctx)
	defer typingCancel()
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				if err := client.MarkReadWithTyping(evt.ID); err != nil {
					log.Printf("relay: failed to refresh typing for %s: %v", evt.ID, err)
				}
			}
		}
	}()

	log.Printf("forwarded message %s from %s [role: %s, session: %s]", evt.ID, evt.From, role, sessionKey)

	msgCtx, msgCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer msgCancel()

	reply, err := gw.SendAndReceive(msgCtx, &gateway.Request{
		SessionKey:     sessionKey,
		IdempotencyKey: evt.ID,
		From:           evt.From,
		FromName:       evt.Name,
		Role:           role,
		Text:           evt.Text,
	})

	typingCancel()

	if err != nil {
		log.Printf("error getting agent reply for %s: %v", evt.ID, err)
		if errorMessage != "" {
			if _, sendErr := client.SendText(from, errorMessage); sendErr != nil {
				log.Printf("relay: failed to send error message to %s: %v", from, sendErr)
			}
		}
		if markErr := client.MarkRead(evt.ID); markErr != nil {
			log.Printf("relay: failed to dismiss typing for %s: %v", evt.ID, markErr)
		}
		return
	}

	// Format and send reply.
	text := gateway.MdToWhatsApp(reply)
	chunks := gateway.SplitMessage(text, 4096)
	for _, chunk := range chunks {
		if _, err := client.SendText(from, chunk); err != nil {
			log.Printf("relay: failed to send WhatsApp chunk to %s: %v", from, err)
		}
	}
	log.Printf("relay: sent %d chunk(s) to %s", len(chunks), from)

	// Dismiss typing indicator.
	if err := client.MarkRead(evt.ID); err != nil {
		log.Printf("relay: failed to dismiss typing for %s: %v", evt.ID, err)
	}
}

// cleanupFunnel gracefully stops the tailscale funnel process if it was started.
func cleanupFunnel(proc *os.Process) {
	if proc == nil {
		return
	}
	log.Printf("stopping tailscale funnel (pid %d)", proc.Pid)
	_ = proc.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("tailscale funnel did not exit, sending SIGKILL")
		_ = proc.Kill()
	}
}
