package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/webhook"
)

func main() {
	addr := envOr("KAPSO_WEBHOOK_ADDR", ":18790")
	webhookSecret := os.Getenv("KAPSO_WEBHOOK_SECRET")
	gatewayURL := envOr("OPENCLAW_GATEWAY_URL", "ws://127.0.0.1:18789")
	gatewayToken := os.Getenv("OPENCLAW_TOKEN")

	if webhookSecret == "" {
		log.Fatal("KAPSO_WEBHOOK_SECRET must be set")
	}

	// Connect to OpenClaw gateway.
	gw := gateway.NewClient(gatewayURL, gatewayToken)
	if err := gw.Connect(); err != nil {
		log.Fatalf("failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	// Start webhook server.
	handler := webhook.NewHandler(gw)
	server := webhook.NewServer(addr, webhookSecret, handler)

	// Graceful shutdown on signal.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("received %s, shutting down", sig)
		gw.Close()
		os.Exit(0)
	}()

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("webhook server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
