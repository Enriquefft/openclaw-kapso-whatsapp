package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

func main() {
	apiKey := os.Getenv("KAPSO_API_KEY")
	phoneNumberID := os.Getenv("KAPSO_PHONE_NUMBER_ID")
	gatewayURL := envOr("OPENCLAW_GATEWAY_URL", "ws://127.0.0.1:18789")
	gatewayToken := os.Getenv("OPENCLAW_TOKEN")
	sessionKey := envOr("OPENCLAW_SESSION_KEY", "main")
	intervalStr := envOr("KAPSO_POLL_INTERVAL", "30")
	stateDir := envOr("KAPSO_STATE_DIR", filepath.Join(os.Getenv("HOME"), ".config", "kapso-whatsapp"))

	if apiKey == "" || phoneNumberID == "" {
		log.Fatal("KAPSO_API_KEY and KAPSO_PHONE_NUMBER_ID must be set")
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
	stateFile := filepath.Join(stateDir, "last-poll")

	// Ensure state dir exists.
	os.MkdirAll(stateDir, 0o700)

	// Load last poll timestamp.
	lastPoll := loadState(stateFile)
	if lastPoll.IsZero() {
		// First run: start from now to avoid replaying history.
		lastPoll = time.Now().UTC()
		saveState(stateFile, lastPoll)
		log.Printf("first run, starting from %s", lastPoll.Format(time.RFC3339))
	}

	log.Printf("polling every %ds, gateway=%s session=%s", interval, gatewayURL, sessionKey)

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Poll immediately on start, then on interval.
	poll(client, gw, sessionKey, stateFile, &lastPoll)

	for {
		select {
		case <-ticker.C:
			poll(client, gw, sessionKey, stateFile, &lastPoll)
		case sig := <-stop:
			log.Printf("received %s, shutting down", sig)
			return
		}
	}
}

func poll(client *kapso.Client, gw *gateway.Client, sessionKey, stateFile string, lastPoll *time.Time) {
	since := lastPoll.Format(time.RFC3339)

	resp, err := client.ListMessages(kapso.ListMessagesParams{
		Direction: "inbound",
		Since:     since,
		Limit:     100,
	})
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	if len(resp.Data) == 0 {
		return
	}

	var newest time.Time
	forwarded := 0

	for _, msg := range resp.Data {
		if msg.Type != "text" || msg.Text == nil {
			continue
		}

		msgTime := parseTimestamp(msg.Timestamp)

		name := ""
		if msg.Kapso != nil {
			name = msg.Kapso.ContactName
		}

		// Build the message text with sender context so the agent knows who
		// to reply to via kapso-whatsapp-cli.
		text := buildMessage(msg.From, name, msg.Text.Body)

		// Use the Kapso message ID as the idempotency key to prevent
		// duplicate deliveries on retries.
		if err := gw.Send(sessionKey, msg.ID, text); err != nil {
			log.Printf("error forwarding message %s: %v", msg.ID, err)
			continue
		}
		forwarded++

		if !msgTime.IsZero() && msgTime.After(newest) {
			newest = msgTime
		}
	}

	if forwarded > 0 {
		log.Printf("forwarded %d message(s)", forwarded)
	}

	// Advance the cursor past the newest message.
	if !newest.IsZero() {
		*lastPoll = newest.Add(time.Second)
		saveState(stateFile, *lastPoll)
	}
}

// buildMessage formats an inbound WhatsApp message for the agent, embedding
// the sender's number and name so the agent can reply to the right contact.
func buildMessage(from, name, body string) string {
	var sb strings.Builder
	sb.WriteString("[WhatsApp from ")
	sb.WriteString(from)
	if name != "" {
		sb.WriteString(" (")
		sb.WriteString(name)
		sb.WriteString(")")
	}
	sb.WriteString("] ")
	sb.WriteString(body)
	return sb.String()
}

// parseTimestamp parses a message timestamp that may be either RFC3339
// (e.g. "2026-01-01T00:00:00Z") or a Unix epoch second string (e.g. "1740704195").
func parseTimestamp(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return time.Unix(n, 0).UTC()
	}
	return time.Time{}
}

func loadState(path string) time.Time {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}
	return t
}

func saveState(path string, t time.Time) {
	os.WriteFile(path, []byte(t.Format(time.RFC3339)), 0o600)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
