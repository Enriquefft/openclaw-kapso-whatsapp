package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/gorilla/websocket"
)

// OpenClaw protocol types.

type requestFrame struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

type responseFrame struct {
	Type   string          `json:"type"`
	ID     string          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type connectParams struct {
	MinProtocol int         `json:"minProtocol"`
	MaxProtocol int         `json:"maxProtocol"`
	Client      clientInfo  `json:"client"`
	Auth        authInfo    `json:"auth"`
	Device      *DeviceInfo `json:"device,omitempty"`
	Role        string      `json:"role"`
	Scopes      []string    `json:"scopes"`
}

// DeviceInfo identifies this device to the gateway via a signed challenge.
type DeviceInfo struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce"`
}

// Signer provides device identity for the gateway connect handshake.
type Signer interface {
	DeviceID() string
	PublicKeyBase64() string
	Sign(nonce string) (signature string, signedAt int64, err error)
}

type clientInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Mode        string `json:"mode"`
}

type authInfo struct {
	Token string `json:"token"`
}

type chatSendParams struct {
	SessionKey     string `json:"sessionKey"`
	Message        string `json:"message"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// Version is the bridge version sent in the connect handshake.
// Overridden at build time via -ldflags.
var Version = "dev"

// maxClaimed caps the replyTracker map size. Entries older than this many
// replies are irrelevant for dedup — the polling window is 10 min.
const maxClaimed = 1000

// replyTracker prevents concurrent relay goroutines from claiming the same
// assistant reply in the session JSONL.
type replyTracker struct {
	mu      sync.Mutex
	claimed map[string]bool
}

func newReplyTracker() *replyTracker {
	return &replyTracker{claimed: make(map[string]bool)}
}

func (rt *replyTracker) claim(key string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.claimed[key] {
		return false
	}
	if len(rt.claimed) >= maxClaimed {
		rt.claimed = make(map[string]bool)
	}
	rt.claimed[key] = true
	return true
}

type assistantReply struct {
	Key  string
	Text string
}

// OpenClaw implements Gateway for the OpenClaw agent runtime.
type OpenClaw struct {
	url          string
	token        string
	signer       Signer
	sessionsJSON string
	sessionKey   string

	conn    *websocket.Conn
	mu      sync.Mutex
	seq     int
	tracker *replyTracker
}

// NewOpenClaw creates an OpenClaw gateway from config.
func NewOpenClaw(cfg config.GatewayConfig) *OpenClaw {
	return &OpenClaw{
		url:          cfg.URL,
		token:        cfg.Token,
		sessionsJSON: cfg.SessionsJSON,
		sessionKey:   cfg.SessionKey,
		tracker:      newReplyTracker(),
	}
}

// NewOpenClawWithSigner creates an OpenClaw gateway with a device identity signer.
func NewOpenClawWithSigner(cfg config.GatewayConfig, signer Signer) *OpenClaw {
	oc := NewOpenClaw(cfg)
	oc.signer = signer
	return oc
}

func (oc *OpenClaw) nextID() string {
	oc.seq++
	return fmt.Sprintf("kapso-%d", oc.seq)
}

// Connect establishes the WebSocket connection and completes the
// challenge-response auth handshake with the OpenClaw gateway.
func (oc *OpenClaw) Connect(ctx context.Context) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, oc.url, nil)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	oc.conn = conn

	// Read the challenge from the gateway.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("read challenge: %w", err)
	}

	log.Printf("received challenge from gateway (%d bytes)", len(msg))

	// Parse challenge to extract nonce for device signing.
	var challenge struct {
		Payload struct {
			Nonce string `json:"nonce"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(msg, &challenge); err != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("parse challenge frame: %w", err)
	}

	// Build device identity if a signer is configured.
	var deviceInfo *DeviceInfo
	if oc.signer != nil {
		nonce := challenge.Payload.Nonce
		if nonce == "" {
			_ = conn.Close()
			oc.conn = nil
			return fmt.Errorf("gateway challenge missing nonce")
		}
		sig, signedAt, err := oc.signer.Sign(nonce)
		if err != nil {
			_ = conn.Close()
			oc.conn = nil
			return fmt.Errorf("sign challenge nonce: %w", err)
		}
		deviceInfo = &DeviceInfo{
			ID:        oc.signer.DeviceID(),
			PublicKey: oc.signer.PublicKeyBase64(),
			Signature: sig,
			SignedAt:  signedAt,
			Nonce:     nonce,
		}
	}

	// Send connect request.
	connectReq := requestFrame{
		Type:   "req",
		ID:     oc.nextID(),
		Method: "connect",
		Params: connectParams{
			MinProtocol: 3,
			MaxProtocol: 3,
			Client: clientInfo{
				ID:          "gateway-client",
				DisplayName: "Kapso WhatsApp Bridge",
				Version:     Version,
				Platform:    runtime.GOOS,
				Mode:        "backend",
			},
			Auth: authInfo{
				Token: oc.token,
			},
			Device: deviceInfo,
			Role:   "operator",
			Scopes: []string{"operator.read", "operator.write"},
		},
	}

	data, err := json.Marshal(connectReq)
	if err != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("marshal connect request: %w", err)
	}

	log.Printf("sending connect request")

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("send connect: %w", err)
	}

	// Wait for response.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("read connect response: %w", err)
	}

	log.Printf("received connect response (%d bytes)", len(msg))

	var resp responseFrame
	if err := json.Unmarshal(msg, &resp); err != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("parse connect response: %w", err)
	}

	if resp.Error != nil {
		_ = conn.Close()
		oc.conn = nil
		return fmt.Errorf("connect rejected: %s", string(resp.Error))
	}

	// Clear deadline for normal operation.
	_ = conn.SetReadDeadline(time.Time{})

	// Drain unsolicited gateway events in the background so the socket
	// buffer never fills up and write operations don't stall.
	go oc.drain()

	log.Printf("authenticated with gateway at %s", oc.url)
	return nil
}

// drain reads and discards all incoming frames from the gateway.
func (oc *OpenClaw) drain() {
	for {
		oc.mu.Lock()
		conn := oc.conn
		oc.mu.Unlock()
		if conn == nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		log.Printf("gateway event (%d bytes)", len(msg))
	}
}

// send submits a message to the OpenClaw gateway via chat.send.
func (oc *OpenClaw) send(sessionKey, idempotencyKey, message string) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.conn == nil {
		return fmt.Errorf("not connected to gateway")
	}

	req := requestFrame{
		Type:   "req",
		ID:     oc.nextID(),
		Method: "chat.send",
		Params: chatSendParams{
			SessionKey:     sessionKey,
			Message:        message,
			IdempotencyKey: idempotencyKey,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	if err := oc.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

// SendAndReceive sends a message to the OpenClaw gateway and polls the
// session JSONL until the agent produces a reply.
func (oc *OpenClaw) SendAndReceive(ctx context.Context, req *Request) (string, error) {
	// Format message with sender metadata — OpenClaw convention.
	taggedText := fmt.Sprintf("From: %s (%s) [role: %s]\n%s",
		req.From, req.FromName, req.Role, req.Text)

	sessionKey := req.SessionKey
	if sessionKey == "" {
		sessionKey = oc.sessionKey
	}

	// Start reply watch *before* sending so fast assistant replies are not missed.
	since := time.Now().UTC().Add(-2 * time.Second)

	if err := oc.send(sessionKey, req.IdempotencyKey, taggedText); err != nil {
		return "", err
	}

	// Poll the session JSONL for the agent's reply.
	deadline := time.Now().Add(10 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for agent reply (session %s)", sessionKey)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}

		sessionFile, err := getSessionFile(oc.sessionsJSON, sessionKey)
		if err != nil {
			log.Printf("openclaw: %v", err)
			continue
		}

		replies, err := getAssistantReplies(sessionFile, since)
		if err != nil {
			log.Printf("openclaw: error reading session: %v", err)
			continue
		}

		for _, reply := range replies {
			if oc.tracker.claim(reply.Key) {
				return reply.Text, nil
			}
		}
	}
}

// Close closes the WebSocket connection.
func (oc *OpenClaw) Close() error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.conn != nil {
		err := oc.conn.Close()
		oc.conn = nil
		return err
	}
	return nil
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
// stopReason=stop that were recorded after `since`.
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
