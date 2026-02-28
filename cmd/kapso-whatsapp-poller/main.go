package main

import (
	"encoding/json"
	"fmt"
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
	sessionsJSON := envOr("OPENCLAW_SESSIONS_JSON",
		filepath.Join(os.Getenv("HOME"), ".openclaw", "agents", "main", "sessions", "sessions.json"))

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
	poll(client, gw, sessionKey, sessionsJSON, stateFile, &lastPoll)

	for {
		select {
		case <-ticker.C:
			poll(client, gw, sessionKey, sessionsJSON, stateFile, &lastPoll)
		case sig := <-stop:
			log.Printf("received %s, shutting down", sig)
			return
		}
	}
}

func poll(client *kapso.Client, gw *gateway.Client, sessionKey, sessionsJSON, stateFile string, lastPoll *time.Time) {
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

		text := buildMessage(msg.From, name, msg.Text.Body)

		// Note the time just before sending so the relay goroutine can find
		// the agent's reply (any assistant stop-message after this time).
		sendAt := time.Now().UTC()

		// Use the Kapso message ID as the idempotency key to prevent
		// duplicate deliveries on retries.
		if err := gw.Send(sessionKey, msg.ID, text); err != nil {
			log.Printf("error forwarding message %s: %v", msg.ID, err)
			continue
		}
		forwarded++

		// Automatically relay the agent's reply back to the WhatsApp sender.
		// This mirrors what the TUI does: the agent just responds in text,
		// and we deliver it — no exec call needed from the agent.
		go waitAndRelay(sessionsJSON, sessionKey, msg.From, sendAt, client)

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

// waitAndRelay polls the session JSONL until the agent produces a reply, then
// sends it back to the WhatsApp sender automatically.
func waitAndRelay(sessionsJSON, sessionKey, from string, since time.Time, client *kapso.Client) {
	to := from
	if !strings.HasPrefix(to, "+") {
		to = "+" + to
	}

	deadline := time.Now().Add(3 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			log.Printf("relay: timeout waiting for agent reply to %s", to)
			return
		}

		<-ticker.C

		sessionFile, err := getSessionFile(sessionsJSON, sessionKey)
		if err != nil {
			log.Printf("relay: %v", err)
			continue
		}

		text, err := getAssistantReply(sessionFile, since)
		if err != nil {
			log.Printf("relay: error reading session: %v", err)
			continue
		}
		if text == "" {
			continue
		}

		if _, err := client.SendText(to, text); err != nil {
			log.Printf("relay: failed to send WhatsApp reply to %s: %v", to, err)
		} else {
			log.Printf("relay: sent reply to %s", to)
		}
		return
	}
}

// getSessionFile reads sessions.json and returns the path to the active
// session JSONL file for the given session key.
func getSessionFile(sessionsJSON, sessionKey string) (string, error) {
	data, err := os.ReadFile(sessionsJSON)
	if err != nil {
		return "", fmt.Errorf("read sessions.json: %w", err)
	}

	var sessions map[string]struct {
		SessionFile string `json:"sessionFile"`
	}
	if err := json.Unmarshal(data, &sessions); err != nil {
		return "", fmt.Errorf("parse sessions.json: %w", err)
	}

	// Try the canonical key first: "agent:KEY:KEY"
	canonical := "agent:" + sessionKey + ":" + sessionKey
	if s, ok := sessions[canonical]; ok && s.SessionFile != "" {
		return s.SessionFile, nil
	}

	// Fall back: first entry whose key contains sessionKey.
	for k, s := range sessions {
		if strings.Contains(k, sessionKey) && s.SessionFile != "" {
			return s.SessionFile, nil
		}
	}

	return "", fmt.Errorf("no session file found for key %q in %s", sessionKey, sessionsJSON)
}

// getAssistantReply scans the session JSONL for the most recent assistant
// message with stopReason=stop that was recorded after `since`.
func getAssistantReply(sessionFile string, since time.Time) (string, error) {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return "", err
	}

	var lastText string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				Role       string `json:"role"`
				StopReason string `json:"stopReason"`
				Content    []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Type != "message" || entry.Timestamp.Before(since) {
			continue
		}
		if entry.Message.Role != "assistant" || entry.Message.StopReason != "stop" {
			continue
		}

		var texts []string
		for _, block := range entry.Message.Content {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
		if len(texts) > 0 {
			lastText = strings.Join(texts, "\n")
		}
	}

	return lastText, nil
}

// buildMessage formats an inbound WhatsApp message for the agent, embedding
// the sender's number and name so the agent has context for who it's talking to.
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
