package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

const waMaxLen = 4096

// Compiled regexes for mdToWhatsApp – compiled once at startup.
var (
	reBold       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic     = regexp.MustCompile(`\*(.+?)\*`)
	reStrike     = regexp.MustCompile(`~~(.+?)~~`)
	reHeading    = regexp.MustCompile(`(?m)^#{1,3} +(.+)$`)
	reBlockquote = regexp.MustCompile(`(?m)^> ?`)
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
		text, ok := extractMessageText(msg, client)
		if !ok {
			continue
		}

		msgTime := parseTimestamp(msg.Timestamp)

		name := ""
		if msg.Kapso != nil {
			name = msg.Kapso.ContactName
		}

		gwText := buildMessage(msg.From, name, text)

		// Note the time just before sending so the relay goroutine can find
		// the agent's reply (any assistant stop-message after this time).
		sendAt := time.Now().UTC()

		// Use the Kapso message ID as the idempotency key to prevent
		// duplicate deliveries on retries.
		if err := gw.Send(sessionKey, msg.ID, gwText); err != nil {
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

// extractMessageText converts an inbound message of any supported type into a
// text representation suitable for forwarding to the gateway. It returns the
// text and true on success, or ("", false) if the message should be skipped.
// Unsupported types are logged and trigger a WhatsApp reply to the sender.
func extractMessageText(msg kapso.InboundMessage, client *kapso.Client) (string, bool) {
	switch msg.Type {
	case "text":
		if msg.Text == nil {
			return "", false
		}
		return msg.Text.Body, true

	case "image":
		if msg.Image == nil {
			return "", false
		}
		return formatMediaMessage("image", msg.Image.Caption, msg.Image.MimeType, msg.Image.ID, client), true

	case "document":
		if msg.Document == nil {
			return "", false
		}
		label := msg.Document.Filename
		if label == "" {
			label = msg.Document.Caption
		}
		return formatMediaMessage("document", label, msg.Document.MimeType, msg.Document.ID, client), true

	case "audio":
		if msg.Audio == nil {
			return "", false
		}
		return formatMediaMessage("audio", "", msg.Audio.MimeType, msg.Audio.ID, client), true

	case "video":
		if msg.Video == nil {
			return "", false
		}
		return formatMediaMessage("video", msg.Video.Caption, msg.Video.MimeType, msg.Video.ID, client), true

	case "location":
		if msg.Location == nil {
			return "", false
		}
		return formatLocationMessage(msg.Location), true

	default:
		log.Printf("unsupported message type %q from %s (id=%s)", msg.Type, msg.From, msg.ID)
		go notifyUnsupported(msg.From, msg.Type, client)
		return "", false
	}
}

// formatMediaMessage builds a text representation for a media attachment.
// It attempts to retrieve the download URL from Kapso and includes it if
// available. The result is always a non-empty string.
func formatMediaMessage(kind, label, mimeType, mediaID string, client *kapso.Client) string {
	var parts []string
	parts = append(parts, "["+kind+"]")
	if label != "" {
		parts = append(parts, label)
	}
	if mimeType != "" {
		parts = append(parts, "("+mimeType+")")
	}

	// Best-effort media URL retrieval — non-fatal if it fails.
	if mediaID != "" && client != nil {
		if media, err := client.GetMediaURL(mediaID); err == nil && media.URL != "" {
			parts = append(parts, media.URL)
		} else if err != nil {
			log.Printf("could not retrieve media URL for %s: %v", mediaID, err)
		}
	}

	return strings.Join(parts, " ")
}

// formatLocationMessage builds a text representation for a location message.
func formatLocationMessage(loc *kapso.LocationContent) string {
	var parts []string
	parts = append(parts, "[location]")
	if loc.Name != "" {
		parts = append(parts, loc.Name)
	}
	if loc.Address != "" {
		parts = append(parts, loc.Address)
	}
	parts = append(parts, fmt.Sprintf("(%.6f, %.6f)", loc.Latitude, loc.Longitude))
	return strings.Join(parts, " ")
}

// notifyUnsupported sends a WhatsApp reply informing the user that their
// message type is not yet supported.
func notifyUnsupported(from, msgType string, client *kapso.Client) {
	to := from
	if !strings.HasPrefix(to, "+") {
		to = "+" + to
	}
	reply := fmt.Sprintf("Sorry, I can't process %s messages yet. Please send text instead.", msgType)
	if _, err := client.SendText(to, reply); err != nil {
		log.Printf("failed to send unsupported-type notice to %s: %v", to, err)
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

		text = mdToWhatsApp(text)
		chunks := splitMessage(text, waMaxLen)
		for _, chunk := range chunks {
			if _, err := client.SendText(to, chunk); err != nil {
				log.Printf("relay: failed to send WhatsApp chunk to %s: %v", to, err)
			}
		}
		log.Printf("relay: sent %d chunk(s) to %s", len(chunks), to)
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

// buildMessage passes through only the raw message body — transport context is
// handled by the bridge, not the agent.
func buildMessage(_, _, body string) string {
	return body
}

// mdToWhatsApp converts Markdown formatting to WhatsApp-compatible formatting.
// Conversion order avoids regex conflicts between bold and italic markers.
func mdToWhatsApp(text string) string {
	const boldMarker = "\x01"

	// 1. Replace **bold** with placeholder to avoid conflicts with *italic*
	result := reBold.ReplaceAllString(text, boldMarker+"$1"+boldMarker)

	// 2. Convert *italic* → _italic_
	result = reItalic.ReplaceAllString(result, "_$1_")

	// 3. Restore bold placeholders as *bold*
	result = strings.ReplaceAll(result, boldMarker, "*")

	// 4. Convert ~~strikethrough~~ → ~strikethrough~
	result = reStrike.ReplaceAllString(result, "~$1~")

	// 5. Convert headings (# / ## / ###) → *Heading*
	result = reHeading.ReplaceAllString(result, "*$1*")

	// 6. Strip blockquote markers
	result = reBlockquote.ReplaceAllString(result, "")

	return result
}

// splitMessage splits text into chunks of at most maxLen bytes, breaking at
// clean boundaries in priority order: paragraph, newline, sentence, word, hard cut.
// Each chunk is trimmed of leading/trailing whitespace.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	minSplit := maxLen / 4
	var chunks []string

	for len(text) > maxLen {
		chunk := text[:maxLen]

		// Priority 1: paragraph break.
		if i := strings.LastIndex(chunk, "\n\n"); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		// Priority 2: single newline.
		if i := strings.LastIndex(chunk, "\n"); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		// Priority 3: sentence ending (includes punctuation, drops trailing space).
		splitPos := -1
		for _, sep := range []string{". ", "? ", "! "} {
			if i := strings.LastIndex(chunk, sep); i >= minSplit {
				pos := i + 1 // include punctuation, exclude the space
				if pos > splitPos {
					splitPos = pos
				}
			}
		}
		if splitPos >= 0 {
			chunks = append(chunks, strings.TrimSpace(text[:splitPos]))
			text = strings.TrimSpace(text[splitPos:])
			continue
		}

		// Priority 4: word boundary.
		if i := strings.LastIndex(chunk, " "); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		// Priority 5: hard cut.
		chunks = append(chunks, strings.TrimSpace(text[:maxLen]))
		text = strings.TrimSpace(text[maxLen:])
	}

	if text != "" {
		chunks = append(chunks, strings.TrimSpace(text))
	}

	return chunks
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
