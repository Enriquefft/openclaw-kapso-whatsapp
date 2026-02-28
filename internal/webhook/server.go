package webhook

import (
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
	"sync"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

// MessageHandler is called for each inbound text message received via webhook.
// Parameters: message ID, sender phone, contact display name, message body, timestamp.
type MessageHandler func(id, from, name, body, timestamp string)

// Server is an HTTP server that receives Meta-format WhatsApp webhook events
// from Kapso and forwards them to a MessageHandler.
type Server struct {
	addr        string
	verifyToken string
	appSecret   string
	handler     MessageHandler
	seen        sync.Map // message ID → struct{}
	srv         *http.Server
}

// NewServer creates a webhook server.
//   - addr: listen address (e.g. ":18790")
//   - verifyToken: token Kapso sends to verify the webhook URL (GET challenge)
//   - appSecret: optional HMAC-SHA256 secret for validating POST payloads
//   - handler: callback for each inbound text message
func NewServer(addr, verifyToken, appSecret string, handler MessageHandler) *Server {
	return &Server{
		addr:        addr,
		verifyToken: verifyToken,
		appSecret:   appSecret,
		handler:     handler,
	}
}

// MarkSeen records a message ID so it won't be processed again.
// Returns true if the ID was already seen.
func (s *Server) MarkSeen(id string) bool {
	_, loaded := s.seen.LoadOrStore(id, struct{}{})
	return loaded
}

// Start begins listening for webhook requests. It blocks until the server is
// stopped or encounters a fatal listener error.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", s.handleWebhook)
	mux.HandleFunc("/health", s.handleHealth)

	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("webhook listen: %w", err)
	}
	log.Printf("webhook server listening on %s", ln.Addr())
	return s.srv.Serve(ln)
}

// handleWebhook processes both verification (GET) and event delivery (POST).
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleVerification(w, r)
	case http.MethodPost:
		s.handleEvent(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVerification responds to Meta's webhook verification challenge.
// Kapso sends: GET /webhook?hub.mode=subscribe&hub.verify_token=TOKEN&hub.challenge=CHALLENGE
func (s *Server) handleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == s.verifyToken {
		log.Printf("webhook verification successful")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, challenge)
		return
	}

	log.Printf("webhook verification failed: mode=%q token_match=%v", mode, token == s.verifyToken)
	http.Error(w, "verification failed", http.StatusForbidden)
}

// handleEvent parses a webhook POST and dispatches each inbound text message.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Validate HMAC signature if app secret is configured.
	if s.appSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !s.validSignature(body, sig) {
			log.Printf("webhook: invalid signature")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload kapso.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("webhook: invalid JSON: %v", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Acknowledge immediately — process asynchronously.
	w.WriteHeader(http.StatusOK)

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
				if msg.Type != "text" || msg.Text == nil {
					continue
				}

				if s.MarkSeen(msg.ID) {
					log.Printf("webhook: skipping duplicate message %s", msg.ID)
					continue
				}

				name := contacts[msg.From]
				s.handler(msg.ID, msg.From, name, msg.Text.Body, msg.Timestamp)
			}
		}
	}
}

// validSignature checks the X-Hub-Signature-256 HMAC.
func (s *Server) validSignature(body []byte, header string) bool {
	if header == "" {
		return false
	}
	sig := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(s.appSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// handleHealth returns 200 OK — used by the CLI status command.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// CleanSeen removes old entries from the seen set. Call periodically to bound
// memory usage. For simplicity we clear the entire map; the worst case is one
// duplicate message right after cleanup.
func (s *Server) CleanSeen() {
	s.seen.Range(func(key, _ interface{}) bool {
		s.seen.Delete(key)
		return true
	})
}
