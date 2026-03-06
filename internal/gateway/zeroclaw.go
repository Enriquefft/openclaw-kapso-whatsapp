package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
	"github.com/gorilla/websocket"
)

// senderConn holds a per-sender WebSocket connection and its I/O mutex.
type senderConn struct {
	conn *websocket.Conn
	ioMu sync.Mutex // serialises write+read cycles on this connection
}

// ZeroClaw implements Gateway for the ZeroClaw agent runtime.
// It communicates via WebSocket at /ws/chat with streaming responses.
//
// Each sender (identified by Request.From) gets a dedicated WebSocket
// connection so that ZeroClaw maintains separate conversation histories
// per user. Messages from different senders never share context.
type ZeroClaw struct {
	url   string
	token string

	mu    sync.Mutex             // guards conns map
	conns map[string]*senderConn // sender key → connection
}

// NewZeroClaw creates a ZeroClaw gateway from config.
func NewZeroClaw(cfg config.GatewayConfig) *ZeroClaw {
	return &ZeroClaw{
		url:   cfg.URL,
		token: cfg.Token,
		conns: make(map[string]*senderConn),
	}
}

// Connect validates that the gateway URL is reachable by establishing a
// probe connection. This keeps the Gateway interface contract (fail-fast
// on startup) while deferring per-sender connections to SendAndReceive.
func (zc *ZeroClaw) Connect(ctx context.Context) error {
	conn, err := zc.dial(ctx)
	if err != nil {
		return err
	}
	// Store as the default connection (used when From is empty, e.g. CLI).
	zc.mu.Lock()
	zc.conns[""] = &senderConn{conn: conn}
	zc.mu.Unlock()

	log.Printf("connected to zeroclaw at %s", zc.url)
	return nil
}

// SendAndReceive sends a message to ZeroClaw and waits for the full response.
// Each unique sender gets a dedicated WebSocket so ZeroClaw maintains
// isolated conversation histories per user.
func (zc *ZeroClaw) SendAndReceive(ctx context.Context, req *Request) (string, error) {
	sc, err := zc.connFor(ctx, req.From)
	if err != nil {
		return "", err
	}

	sc.ioMu.Lock()
	defer sc.ioMu.Unlock()

	// Send message — ZeroClaw takes raw text content.
	msg := map[string]string{
		"type":    "message",
		"content": req.Text,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	if err := sc.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		// Connection broken — remove so next call reconnects.
		zc.removeSender(senderKey(req.From))
		return "", fmt.Errorf("write message: %w", err)
	}

	// Read frames until we get a "done" or "error" response.
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		_, raw, err := sc.conn.ReadMessage()
		if err != nil {
			zc.removeSender(senderKey(req.From))
			return "", fmt.Errorf("read response: %w", err)
		}

		var frame struct {
			Type         string `json:"type"`
			Content      string `json:"content"`
			FullResponse string `json:"full_response"`
			Message      string `json:"message"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			log.Printf("zeroclaw: ignoring unparseable frame: %s", string(raw))
			continue
		}

		switch frame.Type {
		case "done":
			return frame.FullResponse, nil
		case "error":
			return "", fmt.Errorf("zeroclaw agent error: %s", frame.Message)
		case "chunk", "tool_call", "tool_result":
			// Streaming progress — continue reading.
			continue
		default:
			log.Printf("zeroclaw: unknown frame type %q", frame.Type)
			continue
		}
	}
}

// Close closes all per-sender WebSocket connections.
func (zc *ZeroClaw) Close() error {
	zc.mu.Lock()
	defer zc.mu.Unlock()

	var firstErr error
	for key, sc := range zc.conns {
		if err := sc.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(zc.conns, key)
	}
	return firstErr
}

// connFor returns the senderConn for the given sender, creating a new
// WebSocket connection on first use.
func (zc *ZeroClaw) connFor(ctx context.Context, from string) (*senderConn, error) {
	key := senderKey(from)

	zc.mu.Lock()
	sc, ok := zc.conns[key]
	zc.mu.Unlock()
	if ok {
		return sc, nil
	}

	// New sender — open a dedicated connection.
	conn, err := zc.dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect for sender %s: %w", key, err)
	}

	sc = &senderConn{conn: conn}
	zc.mu.Lock()
	// Double-check: another goroutine may have raced us.
	if existing, ok := zc.conns[key]; ok {
		zc.mu.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	zc.conns[key] = sc
	zc.mu.Unlock()

	log.Printf("zeroclaw: opened session for sender %s", key)
	return sc, nil
}

// dial opens a raw WebSocket connection to ZeroClaw.
func (zc *ZeroClaw) dial(ctx context.Context) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := http.Header{}
	if zc.token != "" {
		headers.Set("Authorization", "Bearer "+zc.token)
	}

	conn, _, err := dialer.DialContext(ctx, zc.url, headers)
	if err != nil {
		return nil, fmt.Errorf("connect to zeroclaw: %w", err)
	}
	return conn, nil
}

// removeSender drops a broken connection from the map so the next call
// to connFor will reconnect.
func (zc *ZeroClaw) removeSender(key string) {
	zc.mu.Lock()
	delete(zc.conns, key)
	zc.mu.Unlock()
}

// senderKey normalises a phone number into a map key. Empty From (CLI usage)
// maps to "" which hits the default probe connection from Connect().
func senderKey(from string) string {
	if from == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(from))
	for _, r := range from {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
