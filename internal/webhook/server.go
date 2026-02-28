package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Server is the HTTP webhook receiver.
type Server struct {
	handler       *Handler
	webhookSecret string
	addr          string
}

// NewServer creates a webhook HTTP server.
func NewServer(addr, webhookSecret string, handler *Handler) *Server {
	return &Server{
		handler:       handler,
		webhookSecret: webhookSecret,
		addr:          addr,
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /webhook", s.handleVerification)
	mux.HandleFunc("POST /webhook", s.handleWebhook)
	mux.HandleFunc("GET /health", s.handleHealth)

	log.Printf("webhook server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

// handleVerification handles the Meta webhook verification challenge.
func (s *Server) handleVerification(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == s.webhookSecret {
		log.Printf("webhook verification successful")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, challenge)
		return
	}

	log.Printf("webhook verification failed (mode=%s)", mode)
	http.Error(w, "forbidden", http.StatusForbidden)
}

// handleWebhook receives and processes incoming webhook events.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("error reading request body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Verify HMAC-SHA256 signature if present.
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig != "" {
		if !s.verifySignature(body, sig) {
			log.Printf("invalid webhook signature")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if err := s.handler.HandlePayload(body); err != nil {
		log.Printf("error handling webhook payload: %v", err)
		// Still return 200 to prevent retries.
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// handleHealth is a simple health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// verifySignature checks the HMAC-SHA256 signature.
func (s *Server) verifySignature(body []byte, signature string) bool {
	// Signature format: sha256=<hex>
	if len(signature) < 8 || signature[:7] != "sha256=" {
		return false
	}
	expectedSig := signature[7:]

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(body)
	computedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(computedSig), []byte(expectedSig))
}
