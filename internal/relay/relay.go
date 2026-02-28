package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
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

// Tracker prevents concurrent relay goroutines from claiming the same
// assistant reply in the session JSONL. Each reply is identified by a unique
// key (session file path + line number) and can only be claimed once.
type Tracker struct {
	mu      sync.Mutex
	claimed map[string]bool
}

// NewTracker creates a new relay tracker.
func NewTracker() *Tracker {
	return &Tracker{claimed: make(map[string]bool)}
}

// Claim attempts to exclusively claim a reply identified by key.
// Returns true on success (first caller wins), false if already claimed.
func (rt *Tracker) Claim(key string) bool {
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

// Relay sends agent replies back to WhatsApp senders.
type Relay struct {
	SessionsJSON string
	Client       *kapso.Client
	Tracker      *Tracker
}

// Send polls the session JSONL until the agent produces a reply, then sends it
// back to the WhatsApp sender. It respects ctx cancellation.
func (r *Relay) Send(ctx context.Context, from, sessionKey string, since time.Time) {
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

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		sessionFile, err := getSessionFile(r.SessionsJSON, sessionKey)
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
		for _, reply := range replies {
			if r.Tracker.Claim(reply.Key) {
				text = reply.Text
				break
			}
		}
		if text == "" {
			continue
		}

		text = mdToWhatsApp(text)
		chunks := splitMessage(text, waMaxLen)
		for _, chunk := range chunks {
			if _, err := r.Client.SendText(to, chunk); err != nil {
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
// stopReason=stop that were recorded after `since`. Each reply is tagged with a
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
