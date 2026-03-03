package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/delivery"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/kapso"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/transcribe"
)

// Server is an HTTP webhook receiver that implements delivery.Source.
// It receives both Kapso-native and Meta-format WhatsApp webhook events
// and emits delivery.Event for ALL message types (text, image, document,
// audio, video, location).
type Server struct {
	Addr         string
	VerifyToken  string
	AppSecret    string
	Client       *kapso.Client
	Transcriber  transcribe.Transcriber // nil = transcription disabled
	MaxAudioSize int64
}

// Run starts the webhook HTTP server and emits events on out. It blocks until
// ctx is cancelled, at which point the server is gracefully shut down.
func (s *Server) Run(ctx context.Context, out chan<- delivery.Event) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", s.webhookHandler(out))
	mux.HandleFunc("/health", handleHealth)

	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("webhook listen: %w", err)
	}
	log.Printf("webhook server listening on %s", ln.Addr())

	// Shut down gracefully when ctx is cancelled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webhook serve: %w", err)
	}
	return nil
}

// webhookHandler returns an http.HandlerFunc that processes both verification
// (GET) and event delivery (POST).
func (s *Server) webhookHandler(out chan<- delivery.Event) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleVerification(w, r)
		case http.MethodPost:
			s.handleEvent(w, r, out)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleVerification responds to Meta's webhook verification challenge.
func (s *Server) handleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == s.VerifyToken {
		log.Printf("webhook verification successful")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, challenge)
		return
	}

	log.Printf("webhook verification failed: mode=%q token_match=%v", mode, token == s.VerifyToken)
	http.Error(w, "verification failed", http.StatusForbidden)
}

// webhookFormat identifies whether the incoming JSON is Kapso native or Meta format.
type webhookFormat int

const (
	formatUnknown webhookFormat = iota
	formatMeta
	formatKapso
)

// detectFormat peeks at the JSON to determine webhook format.
// Kapso native has "type" at top level; Meta has "object" and "entry".
func detectFormat(body []byte) webhookFormat {
	var probe struct {
		Type   string `json:"type"`
		Object string `json:"object"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return formatUnknown
	}
	if probe.Type != "" {
		return formatKapso
	}
	if probe.Object != "" {
		return formatMeta
	}
	return formatUnknown
}

func formatName(f webhookFormat) string {
	switch f {
	case formatKapso:
		return "kapso-native"
	case formatMeta:
		return "meta"
	default:
		return "unknown"
	}
}

// handleEvent parses a webhook POST, detects the payload format (Kapso native
// or Meta), and emits events for inbound messages.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request, out chan<- delivery.Event) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Validate HMAC signature if app secret is configured.
	// Supports both Kapso (X-Webhook-Signature) and Meta (X-Hub-Signature-256).
	if s.AppSecret != "" {
		if !validateSignature(r.Header, body, s.AppSecret) {
			log.Printf("webhook: invalid signature")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	format := detectFormat(body)
	log.Printf("webhook: detected %s format", formatName(format))

	// Acknowledge immediately — process asynchronously.
	w.WriteHeader(http.StatusOK)

	switch format {
	case formatKapso:
		s.handleKapsoPayload(body, out)
	case formatMeta:
		s.handleMetaPayload(body, out)
	default:
		log.Printf("webhook: unrecognized payload format, ignoring")
	}
}

// handleMetaPayload processes a Meta-format webhook payload.
func (s *Server) handleMetaPayload(body []byte, out chan<- delivery.Event) {
	var payload kapso.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("webhook: invalid Meta JSON: %v", err)
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			// Build a contact-name lookup from the contacts array.
			contacts := make(map[string]string)
			for _, c := range change.Value.Contacts {
				contacts[c.WaID] = c.Profile.Name
			}

			for _, msg := range change.Value.Messages {
				s.emitMessage(msg, contacts, nil, out)
			}
		}
	}
}

// handleKapsoPayload processes a Kapso-native webhook payload.
func (s *Server) handleKapsoPayload(body []byte, out chan<- delivery.Event) {
	var payload kapso.KapsoWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("webhook: invalid Kapso JSON: %v", err)
		return
	}

	// Only process message-received events; ignore status updates.
	if payload.Type != "whatsapp.message.received" {
		log.Printf("webhook: ignoring Kapso event type %q", payload.Type)
		return
	}

	for _, item := range payload.Data {
		s.emitMessage(item.Message, nil, item.Conversation, out)
	}
}

// emitMessage extracts text from a message and emits it as a delivery.Event.
// contacts is an optional Meta-format contact-name lookup (nil for Kapso native).
func (s *Server) emitMessage(msg kapso.Message, contacts map[string]string, eventConversation *kapso.KapsoConversation, out chan<- delivery.Event) {
	text, ok := delivery.ExtractText(msg, s.Client, s.Transcriber, s.MaxAudioSize)
	if !ok {
		return
	}

	name := ""
	if msg.Kapso != nil && msg.Kapso.ContactName != "" {
		name = msg.Kapso.ContactName
	} else if contacts != nil {
		name = contacts[msg.From]
	}

	// Extract conversation ID for group detection.
	// Prefer message-level kapso.conversation; fall back to event-level conversation
	// (Kapso-native webhooks carry conversation as a sibling of the message).
	conversationID := ""
	if msg.Kapso != nil && msg.Kapso.Conversation != nil {
		conversationID = msg.Kapso.Conversation.ID
	} else if eventConversation != nil {
		conversationID = eventConversation.ID
	}

	out <- delivery.Event{
		ID:             msg.ID,
		From:           msg.From,
		Name:           name,
		Text:           text,
		ConversationID: conversationID,
	}
	log.Printf("webhook: received message %s from %s", msg.ID, msg.From)
}

// validateSignature checks HMAC-SHA256 for either Kapso or Meta webhook format.
// Kapso: X-Webhook-Signature header, raw hex.
// Meta:  X-Hub-Signature-256 header, "sha256=" prefixed hex.
func validateSignature(headers http.Header, body []byte, secret string) bool {
	// Try Kapso header first.
	if sig := headers.Get("X-Webhook-Signature"); sig != "" {
		return hmacValid(body, sig, secret)
	}
	// Fall back to Meta header.
	if sig := headers.Get("X-Hub-Signature-256"); sig != "" {
		return hmacValid(body, strings.TrimPrefix(sig, "sha256="), secret)
	}
	return false
}

// hmacValid checks if the hex-encoded HMAC-SHA256 of body matches expected.
func hmacValid(body []byte, hexSig, secret string) bool {
	if hexSig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(hexSig), []byte(expected))
}

// handleHealth returns 200 OK — used by the CLI status command.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "ok")
}
