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
)

// Server is an HTTP webhook receiver that implements delivery.Source.
// It receives Meta-format WhatsApp webhook events from Kapso and emits
// delivery.Event for ALL message types (text, image, document, audio, video, location).
type Server struct {
	Addr        string
	VerifyToken string
	AppSecret   string
	Client      *kapso.Client
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
		srv.Shutdown(shutdownCtx)
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
		fmt.Fprint(w, challenge)
		return
	}

	log.Printf("webhook verification failed: mode=%q token_match=%v", mode, token == s.VerifyToken)
	http.Error(w, "verification failed", http.StatusForbidden)
}

// handleEvent parses a webhook POST and emits events for ALL inbound message types.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request, out chan<- delivery.Event) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Validate HMAC signature if app secret is configured.
	if s.AppSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !validSignature(body, sig, s.AppSecret) {
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
				text, ok := delivery.ExtractText(msg, s.Client)
				if !ok {
					continue
				}

				name := contacts[msg.From]
				out <- delivery.Event{
					ID:   msg.ID,
					From: msg.From,
					Name: name,
					Text: text,
				}
				log.Printf("webhook: received message %s from %s", msg.ID, msg.From)
			}
		}
	}
}

// validSignature checks the X-Hub-Signature-256 HMAC.
func validSignature(body []byte, header, secret string) bool {
	if header == "" {
		return false
	}
	sig := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// handleHealth returns 200 OK — used by the CLI status command.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
