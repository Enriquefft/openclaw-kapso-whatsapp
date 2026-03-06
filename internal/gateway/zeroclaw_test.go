package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// TestZeroClawSendAndReceive verifies the full send→done flow.
func TestZeroClawSendAndReceive(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	receivedMsg := make(chan string, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// Read the client's message.
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		receivedMsg <- string(msg)

		// Send a chunk then a done frame.
		chunk := `{"type":"chunk","content":"Hello "}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(chunk))

		doneFrame := `{"type":"done","full_response":"Hello world"}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(doneFrame))

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	zc := &ZeroClaw{url: wsURL, token: "test-token", conns: make(map[string]*senderConn)}

	if err := zc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = zc.Close() }()

	reply, err := zc.SendAndReceive(context.Background(), &Request{
		Text:           "Hi",
		IdempotencyKey: "msg-1",
	})
	if err != nil {
		t.Fatalf("SendAndReceive() failed: %v", err)
	}

	if reply != "Hello world" {
		t.Errorf("expected reply %q, got %q", "Hello world", reply)
	}

	// Verify the message sent to the server.
	raw := <-receivedMsg
	var sent struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &sent); err != nil {
		t.Fatalf("unmarshal sent message: %v", err)
	}
	if sent.Type != "message" {
		t.Errorf("expected type 'message', got %q", sent.Type)
	}
	if sent.Content != "Hi" {
		t.Errorf("expected content 'Hi', got %q", sent.Content)
	}
}

// TestZeroClawErrorResponse verifies that an error frame returns an error.
func TestZeroClawErrorResponse(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		_, _, _ = conn.ReadMessage()

		errFrame := `{"type":"error","message":"rate limit exceeded"}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(errFrame))

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	zc := &ZeroClaw{url: wsURL, conns: make(map[string]*senderConn)}

	if err := zc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = zc.Close() }()

	_, err := zc.SendAndReceive(context.Background(), &Request{Text: "Hi"})
	if err == nil {
		t.Fatal("expected error from error frame")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("error should contain server message, got: %v", err)
	}
}

// TestZeroClawConnectSendsAuthToken verifies the bearer token is sent in the
// Authorization header during the WebSocket handshake.
func TestZeroClawConnectSendsAuthToken(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	gotToken := make(chan string, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		gotToken <- auth

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	wantToken := "my-secret-zeroclaw-token"
	zc := &ZeroClaw{url: wsURL, token: wantToken, conns: make(map[string]*senderConn)}

	if err := zc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = zc.Close() }()

	token := <-gotToken
	expected := fmt.Sprintf("Bearer %s", wantToken)
	if token != expected {
		t.Errorf("expected Authorization %q, got %q", expected, token)
	}
}

// TestZeroClawSessionIsolation verifies that different senders get separate
// WebSocket connections, ensuring conversation history isolation.
func TestZeroClawSessionIsolation(t *testing.T) {
	var upgrader = websocket.Upgrader{}

	// Track per-connection message history to prove isolation.
	type connHistory struct {
		mu       sync.Mutex
		messages []string
	}
	allConns := struct {
		mu    sync.Mutex
		count int
		byID  map[int]*connHistory
	}{byID: make(map[int]*connHistory)}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}

		allConns.mu.Lock()
		allConns.count++
		id := allConns.count
		hist := &connHistory{}
		allConns.byID[id] = hist
		allConns.mu.Unlock()

		defer func() { _ = conn.Close() }()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var sent struct {
				Content string `json:"content"`
			}
			_ = json.Unmarshal(msg, &sent)

			hist.mu.Lock()
			hist.messages = append(hist.messages, sent.Content)
			hist.mu.Unlock()

			// Echo back which connection handled it.
			reply := fmt.Sprintf(`{"type":"done","full_response":"conn-%d: %s"}`, id, sent.Content)
			_ = conn.WriteMessage(websocket.TextMessage, []byte(reply))
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	zc := &ZeroClaw{url: wsURL, conns: make(map[string]*senderConn)}

	if err := zc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = zc.Close() }()

	ctx := context.Background()

	// Sender A sends a message.
	replyA1, err := zc.SendAndReceive(ctx, &Request{
		From: "+51926689401",
		Text: "Hola soy A",
	})
	if err != nil {
		t.Fatalf("sender A msg 1: %v", err)
	}

	// Sender B sends a message.
	replyB1, err := zc.SendAndReceive(ctx, &Request{
		From: "+51984089340",
		Text: "Hola soy B",
	})
	if err != nil {
		t.Fatalf("sender B msg 1: %v", err)
	}

	// Sender A sends another message.
	replyA2, err := zc.SendAndReceive(ctx, &Request{
		From: "+51926689401",
		Text: "Segundo mensaje de A",
	})
	if err != nil {
		t.Fatalf("sender A msg 2: %v", err)
	}

	// Verify they hit different connections.
	// A's messages should be on the same connection (conn-2, since conn-1 is the probe).
	// B's messages should be on a different connection (conn-3).
	if !strings.HasPrefix(replyA1, "conn-2") {
		t.Errorf("sender A msg 1 expected conn-2, got: %s", replyA1)
	}
	if !strings.HasPrefix(replyB1, "conn-3") {
		t.Errorf("sender B msg 1 expected conn-3, got: %s", replyB1)
	}
	if !strings.HasPrefix(replyA2, "conn-2") {
		t.Errorf("sender A msg 2 should reuse conn-2, got: %s", replyA2)
	}

	// Verify total connection count: 1 probe + 2 senders = 3.
	allConns.mu.Lock()
	totalConns := allConns.count
	allConns.mu.Unlock()
	if totalConns != 3 {
		t.Errorf("expected 3 connections (probe + 2 senders), got %d", totalConns)
	}

	// Verify per-connection message history (proves no cross-contamination).
	allConns.mu.Lock()
	connAHist := allConns.byID[2]
	connBHist := allConns.byID[3]
	allConns.mu.Unlock()

	connAHist.mu.Lock()
	if len(connAHist.messages) != 2 {
		t.Errorf("sender A conn should have 2 messages, got %d", len(connAHist.messages))
	}
	connAHist.mu.Unlock()

	connBHist.mu.Lock()
	if len(connBHist.messages) != 1 {
		t.Errorf("sender B conn should have 1 message, got %d", len(connBHist.messages))
	}
	connBHist.mu.Unlock()
}

// TestZeroClawSameSenderReuseConn verifies that the same sender reuses
// their existing WebSocket connection across multiple messages.
func TestZeroClawSameSenderReuseConn(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	var connCount int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		mu.Lock()
		connCount++
		mu.Unlock()

		defer func() { _ = conn.Close() }()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			done := `{"type":"done","full_response":"ok"}`
			_ = conn.WriteMessage(websocket.TextMessage, []byte(done))
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	zc := &ZeroClaw{url: wsURL, conns: make(map[string]*senderConn)}

	if err := zc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = zc.Close() }()

	ctx := context.Background()
	sender := "+51926689401"

	for i := 0; i < 5; i++ {
		_, err := zc.SendAndReceive(ctx, &Request{
			From: sender,
			Text: fmt.Sprintf("msg %d", i),
		})
		if err != nil {
			t.Fatalf("message %d failed: %v", i, err)
		}
	}

	// 1 probe from Connect() + 1 for the sender = 2 total.
	mu.Lock()
	got := connCount
	mu.Unlock()
	if got != 2 {
		t.Errorf("expected 2 connections (probe + 1 sender), got %d", got)
	}
}

// TestSenderKey verifies phone number normalisation for map keys.
func TestSenderKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+51926689401", "51926689401"},
		{"51926689401", "51926689401"},
		{"+1 (555) 123-4567", "15551234567"},
		{"", ""},
	}

	for _, tt := range tests {
		got := senderKey(tt.input)
		if got != tt.want {
			t.Errorf("senderKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
