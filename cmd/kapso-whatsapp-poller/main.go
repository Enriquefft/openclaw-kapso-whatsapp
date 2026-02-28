package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/tailscale"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/webhook"
)

const waMaxLen = 4096

// Compiled regexes for mdToWhatsApp compiled once at startup.
var (
	reBold       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic     = regexp.MustCompile(`\*(.+?)\*`)
	reStrike     = regexp.MustCompile(`~~(.+?)~~`)
	reHeading    = regexp.MustCompile("(?m)^#{1,3} +(.+)$")
	reBlockquote = regexp.MustCompile("(?m)^> ?")
)

// relayTracker prevents concurrent relay goroutines from claiming the same
// assistant reply in the session JSONL. Each reply is identified by a unique
// key (session file path + line number) and can only be claimed once.
type relayTracker struct {
	mu      sync.Mutex
	claimed map[string]bool
}

func newRelayTracker() *relayTracker {
	return &relayTracker{claimed: make(map[string]bool)}
}

// claim attempts to exclusively claim a reply identified by key.
// Returns true on success (first caller wins), false if already claimed.
func (rt *relayTracker) claim(key string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.claimed[key] {
		return false
	}
	rt.claimed[key] = true
	return true
}

// assistantReply pairs a unique claim key with the reply text.
type assistantReply struct {
	Key  string
	Text string
}

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
	// Backward compat: KAPSO_WEBHOOK_MODE "off"->"polling", "webhook"/"both" still work.
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

	tracker := newRelayTracker()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start webhook server if mode requires it.
	var whSrv *webhook.Server
	var funnelProc *os.Process

	if mode == "tailscale" || mode == "domain" {
		whSrv = webhook.NewServer(webhookAddr, webhookVerifyToken, webhookSecret,
			func(id, from, name, body, timestamp string) {
				text := buildMessage(from, name, body)
				sendAt := time.Now().UTC()

				if err := gw.Send(sessionKey, id, text); err != nil {
					log.Printf("webhook: error forwarding message %s: %v", id, err)
					return
				}
				log.Printf("webhook: forwarded message %s from %s", id, from)
				go waitAndRelay(sessionsJSON, sessionKey, from, sendAt, client, tracker)
			})

		go func() {
			if err := whSrv.Start(); err != nil {
				log.Printf("webhook server error: %v", err)
			}
		}()

		// Periodically clean the dedup set (every 10 minutes).
		go func() {
			t := time.NewTicker(10 * time.Minute)
			defer t.Stop()
			for range t.C {
				whSrv.CleanSeen()
			}
		}()

		// In tailscale mode, auto-start Tailscale Funnel.
		if mode == "tailscale" {
			_, port, err := net.SplitHostPort(webhookAddr)
			if err != nil {
				// webhookAddr might be just ":18790" or "18790".
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

	// Determine if polling should run.
	runPolling := mode == "polling" || pollFallback

	if runPolling {
		log.Printf("polling every %ds, gateway=%s session=%s", interval, gatewayURL, sessionKey)
	}

	if !runPolling {
		if mode == "tailscale" {
			log.Printf("tailscale mode, polling disabled, gateway=%s session=%s", gatewayURL, sessionKey)
		} else {
			log.Printf("domain mode, polling disabled, gateway=%s session=%s", gatewayURL, sessionKey)
		}
		// Block until shutdown signal.
		sig := <-stop
		log.Printf("received %s, shutting down", sig)
		cleanupFunnel(funnelProc)
		return
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Poll immediately on start, then on interval.
	poll(client, gw, sessionKey, sessionsJSON, stateFile, &lastPoll, whSrv, tracker)

	for {
		select {
		case <-ticker.C:
			poll(client, gw, sessionKey, sessionsJSON, stateFile, &lastPoll, whSrv, tracker)
		case sig := <-stop:
			log.Printf("received %s, shutting down", sig)
			cleanupFunnel(funnelProc)
			return
		}
	}
}

func poll(client *kapso.Client, gw *gateway.Client, sessionKey, sessionsJSON, stateFile string, lastPoll *time.Time, whSrv *webhook.Server, tracker *relayTracker) {
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

		// Skip messages already processed by webhook (when running with poll fallback).
		if whSrv != nil && whSrv.MarkSeen(msg.ID) {
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
		go waitAndRelay(sessionsJSON, sessionKey, msg.From, sendAt, client, tracker)

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
func formatMediaMessage(kind, label, mimeType, mediaID string, client *kapso.Client) string {
	var parts []string
	parts = append(parts, "["+kind+"]")
	if label != "" {
		parts = append(parts, label)
	}
	if mimeType != "" {
		parts = append(parts, "("+mimeType+")")
	}

	// Best-effort media URL retrieval.
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
func waitAndRelay(sessionsJSON, sessionKey, from string, since time.Time, client *kapso.Client, tracker *relayTracker) {
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

		replies, err := getAssistantReplies(sessionFile, since)
		if err != nil {
			log.Printf("relay: error reading session: %v", err)
			continue
		}

		var text string
		for _, r := range replies {
			if tracker.claim(r.Key) {
				text = r.Text
				break
			}
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

// getAssistantReplies scans the session JSONL for all assistant messages with
// stopReason=stop that were recorded after since. Each reply is tagged with a
// unique key (file path + line number) so relay goroutines can claim exactly one.
func getAssistantReplies(sessionFile string, since time.Time) ([]assistantReply, error) {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, err
	}

	var replies []assistantReply
	for i, line := range strings.Split(string(data), "\n") {
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
			replies = append(replies, assistantReply{
				Key:  fmt.Sprintf("%s:%d", sessionFile, i),
				Text: strings.Join(texts, "\n"),
			})
		}
	}

	return replies, nil
}

// buildMessage passes through only the raw message body.
func buildMessage(_, _, body string) string {
	return body
}

// mdToWhatsApp converts Markdown formatting to WhatsApp-compatible formatting.
func mdToWhatsApp(text string) string {
	const boldMarker = "\x01"

	result := reBold.ReplaceAllString(text, boldMarker+"$1"+boldMarker)
	result = reItalic.ReplaceAllString(result, "_$1_")
	result = strings.ReplaceAll(result, boldMarker, "*")
	result = reStrike.ReplaceAllString(result, "~$1~")
	result = reHeading.ReplaceAllString(result, "*$1*")
	result = reBlockquote.ReplaceAllString(result, "")

	return result
}

// splitMessage splits text into chunks of at most maxLen bytes.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	minSplit := maxLen / 4
	var chunks []string

	for len(text) > maxLen {
		chunk := text[:maxLen]

		if i := strings.LastIndex(chunk, "\n\n"); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		if i := strings.LastIndex(chunk, "\n"); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		splitPos := -1
		for _, sep := range []string{". ", "? ", "! "} {
			if i := strings.LastIndex(chunk, sep); i >= minSplit {
				pos := i + 1
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

		if i := strings.LastIndex(chunk, " "); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		chunks = append(chunks, strings.TrimSpace(text[:maxLen]))
		text = strings.TrimSpace(text[maxLen:])
	}

	if text != "" {
		chunks = append(chunks, strings.TrimSpace(text))
	}

	return chunks
}

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
