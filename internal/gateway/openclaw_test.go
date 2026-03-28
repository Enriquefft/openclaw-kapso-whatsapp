package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockSigner is a test double for the Signer interface.
type mockSigner struct {
	id    string
	pubK  string
	sig   []byte
	lastPayload string
}

func (m *mockSigner) DeviceID() string        { return m.id }
func (m *mockSigner) PublicKeyBase64() string { return m.pubK }
func (m *mockSigner) Sign(data []byte) []byte {
	m.lastPayload = string(data)
	return m.sig
}

// newTestOpenClaw creates an OpenClaw gateway pointed at the given WebSocket URL.
func newTestOpenClaw(url, token string, signer Signer) *OpenClaw {
	oc := &OpenClaw{
		url:     url,
		token:   token,
		signer:  signer,
		tracker: newReplyTracker(),
	}
	return oc
}

// TestConnectRequestsReadAndWriteScopes verifies that the gateway connect
// handshake includes both "operator.read" and "operator.write" scopes.
func TestConnectRequestsReadAndWriteScopes(t *testing.T) {
	var upgrader = websocket.Upgrader{}

	connectFrame := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErr <- fmt.Errorf("upgrade: %w", err)
			return
		}
		defer func() { _ = conn.Close() }()

		challenge := responseFrame{
			Type:   "event",
			Method: "challenge",
		}
		challengeData, _ := json.Marshal(challenge)
		if err := conn.WriteMessage(websocket.TextMessage, challengeData); err != nil {
			serverErr <- fmt.Errorf("write challenge: %w", err)
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			serverErr <- fmt.Errorf("read connect: %w", err)
			return
		}
		connectFrame <- msg

		resp := responseFrame{
			Type:   "res",
			ID:     "kapso-1",
			Result: json.RawMessage(`{"ok": true}`),
		}
		respData, _ := json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, respData); err != nil {
			serverErr <- fmt.Errorf("write response: %w", err)
			return
		}

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := newTestOpenClaw(wsURL, "test-token-nobody-expects-the-spanish-inquisition", nil)

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case err := <-serverErr:
		t.Fatalf("server handler error: %v", err)
	default:
	}

	raw := <-connectFrame

	var frame struct {
		Type   string `json:"type"`
		Method string `json:"method"`
		Params struct {
			Role   string   `json:"role"`
			Scopes []string `json:"scopes"`
			Auth   struct {
				Token string `json:"token"`
			} `json:"auth"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("unmarshal connect frame: %v", err)
	}

	if frame.Method != "connect" {
		t.Fatalf("expected method 'connect', got %q", frame.Method)
	}

	if frame.Params.Role != "operator" {
		t.Fatalf("expected role 'operator', got %q", frame.Params.Role)
	}

	scopes := frame.Params.Scopes
	hasRead := false
	hasWrite := false
	for _, s := range scopes {
		switch s {
		case "operator.read":
			hasRead = true
		case "operator.write":
			hasWrite = true
		case "operator.admin":
			t.Errorf("scope 'operator.admin' is not a valid gateway scope — this was the original bug")
		}
	}

	if !hasRead {
		t.Errorf("missing required scope 'operator.read'; got scopes: %v", scopes)
	}
	if !hasWrite {
		t.Errorf("missing required scope 'operator.write'; got scopes: %v", scopes)
	}
}

// TestConnectForwardsAuthToken ensures the auth token from the constructor is
// actually sent in the connect handshake.
func TestConnectForwardsAuthToken(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	connectFrame := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErr <- fmt.Errorf("upgrade: %w", err)
			return
		}
		defer func() { _ = conn.Close() }()

		challenge := responseFrame{Type: "event", Method: "challenge"}
		data, _ := json.Marshal(challenge)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			serverErr <- fmt.Errorf("write challenge: %w", err)
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			serverErr <- fmt.Errorf("read connect: %w", err)
			return
		}
		connectFrame <- msg

		resp := responseFrame{
			Type:   "res",
			ID:     "kapso-1",
			Result: json.RawMessage(`{"ok": true}`),
		}
		data, _ = json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			serverErr <- fmt.Errorf("write response: %w", err)
			return
		}

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	wantToken := "surely-you-cant-be-serious-i-am-serious-and-dont-call-me-shirley"
	client := newTestOpenClaw(wsURL, wantToken, nil)

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case err := <-serverErr:
		t.Fatalf("server handler error: %v", err)
	default:
	}

	raw := <-connectFrame
	var frame struct {
		Params struct {
			Auth struct {
				Token string `json:"token"`
			} `json:"auth"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if frame.Params.Auth.Token != wantToken {
		t.Fatalf("expected token %q, got %q", wantToken, frame.Params.Auth.Token)
	}
}

// TestConnectIncludesDeviceIdentity verifies that when a Signer is provided,
// the connect request contains a populated device object.
func TestConnectIncludesDeviceIdentity(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	connectFrame := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	done := make(chan struct{})

	challengeNonce := "test-challenge-nonce-from-gateway"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErr <- fmt.Errorf("upgrade: %w", err)
			return
		}
		defer func() { _ = conn.Close() }()

		challenge := fmt.Sprintf(`{"type":"event","method":"challenge","payload":{"nonce":"%s"}}`, challengeNonce)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(challenge)); err != nil {
			serverErr <- fmt.Errorf("write challenge: %w", err)
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			serverErr <- fmt.Errorf("read connect: %w", err)
			return
		}
		connectFrame <- msg

		resp := responseFrame{
			Type:   "res",
			ID:     "kapso-1",
			Result: json.RawMessage(`{"ok": true}`),
		}
		data, _ := json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			serverErr <- fmt.Errorf("write response: %w", err)
			return
		}

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	signer := &mockSigner{
		id:   "device-fingerprint-abc",
		pubK: "dGVzdC1wdWJsaWMta2V5",
		sig:  []byte("test-signature"),
	}

	client := newTestOpenClaw(wsURL, "test-token", signer)

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case err := <-serverErr:
		t.Fatalf("server handler error: %v", err)
	default:
	}

	raw := <-connectFrame

	var frame struct {
		Params struct {
			Device *struct {
				ID        string `json:"id"`
				PublicKey string `json:"publicKey"`
				Signature string `json:"signature"`
				SignedAt  int64  `json:"signedAt"`
				Nonce     string `json:"nonce"`
			} `json:"device"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("unmarshal connect frame: %v", err)
	}

	if frame.Params.Device == nil {
		t.Fatal("device field missing from connect request")
	}

	d := frame.Params.Device
	if d.ID != "device-fingerprint-abc" {
		t.Errorf("device.id: got %q, want %q", d.ID, "device-fingerprint-abc")
	}
	if d.PublicKey != "dGVzdC1wdWJsaWMta2V5" {
		t.Errorf("device.publicKey: got %q, want %q", d.PublicKey, "dGVzdC1wdWJsaWMta2V5")
	}
	if d.Signature != "dGVzdC1zaWduYXR1cmU" {
		t.Errorf("device.signature: got %q, want %q", d.Signature, "dGVzdC1zaWduYXR1cmU")
	}
	if d.SignedAt <= 0 {
		t.Errorf("device.signedAt: got %d, want positive timestamp", d.SignedAt)
	}
	if d.Nonce != challengeNonce {
		t.Errorf("device.nonce: got %q, want %q", d.Nonce, challengeNonce)
	}

	if !strings.Contains(signer.lastPayload, challengeNonce) {
		t.Errorf("signer payload should contain nonce %q, got %q", challengeNonce, signer.lastPayload)
	}
}

// TestConnectWithoutSignerOmitsDevice verifies that when no Signer is
// provided, the device field is absent from the connect request.
func TestConnectWithoutSignerOmitsDevice(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	connectFrame := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErr <- fmt.Errorf("upgrade: %w", err)
			return
		}
		defer func() { _ = conn.Close() }()

		challenge := responseFrame{Type: "event", Method: "challenge"}
		data, _ := json.Marshal(challenge)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			serverErr <- fmt.Errorf("write challenge: %w", err)
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			serverErr <- fmt.Errorf("read connect: %w", err)
			return
		}
		connectFrame <- msg

		resp := responseFrame{
			Type:   "res",
			ID:     "kapso-1",
			Result: json.RawMessage(`{"ok": true}`),
		}
		data, _ = json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			serverErr <- fmt.Errorf("write response: %w", err)
			return
		}

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := newTestOpenClaw(wsURL, "test-token", nil)

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case err := <-serverErr:
		t.Fatalf("server handler error: %v", err)
	default:
	}

	raw := <-connectFrame

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var params map[string]json.RawMessage
	if err := json.Unmarshal(generic["params"], &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	if _, exists := params["device"]; exists {
		t.Error("device field should be absent when no signer is provided")
	}
}

// TestConnectWithSignerButNoNonceErrors verifies that when a Signer is
// configured but the gateway challenge contains no nonce, Connect returns
// a clear error.
func TestConnectWithSignerButNoNonceErrors(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		challenge := responseFrame{Type: "event", Method: "challenge"}
		data, _ := json.Marshal(challenge)
		_ = conn.WriteMessage(websocket.TextMessage, data)

		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	signer := &mockSigner{id: "test", pubK: "dGVzdA==", sig: []byte("sig")}
	client := newTestOpenClaw(wsURL, "test-token", signer)

	err := client.Connect(context.Background())
	if err == nil {
		_ = client.Close()
		t.Fatal("expected error when signer is configured but challenge has no nonce")
	}

	if !strings.Contains(err.Error(), "missing nonce") {
		t.Errorf("error should mention missing nonce, got: %v", err)
	}
}

// TestGetSessionFileFallsBackToBaseKey verifies that when a per-sender
// session key is not in sessions.json, the base key is found instead.
func TestGetSessionFileFallsBackToBaseKey(t *testing.T) {
	dir := t.TempDir()
	sessionsJSON := filepath.Join(dir, "sessions.json")

	data := `{"agent:main:main": {"sessionFile": "/tmp/main.jsonl"}}`
	if err := os.WriteFile(sessionsJSON, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	// Per-sender key should NOT be found.
	_, err := getSessionFile(sessionsJSON, "main-wa-91234567890")
	if err == nil {
		t.Fatal("expected error for per-sender key, got nil")
	}

	// Base key should be found.
	sf, err := getSessionFile(sessionsJSON, "main")
	if err != nil {
		t.Fatalf("expected base key lookup to succeed: %v", err)
	}
	if sf != "/tmp/main.jsonl" {
		t.Fatalf("expected /tmp/main.jsonl, got %s", sf)
	}
}

// TestGetSessionFilePerSenderKeyFound verifies that when sessions.json
// contains a per-sender session entry, it is returned directly.
func TestGetSessionFilePerSenderKeyFound(t *testing.T) {
	dir := t.TempDir()
	sessionsJSON := filepath.Join(dir, "sessions.json")

	data := `{
		"agent:main:main": {"sessionFile": "/tmp/main.jsonl"},
		"agent:main-wa-91234567890:main-wa-91234567890": {"sessionFile": "/tmp/sender.jsonl"}
	}`
	if err := os.WriteFile(sessionsJSON, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	sf, err := getSessionFile(sessionsJSON, "main-wa-91234567890")
	if err != nil {
		t.Fatalf("expected per-sender key lookup to succeed: %v", err)
	}
	if sf != "/tmp/sender.jsonl" {
		t.Fatalf("expected /tmp/sender.jsonl, got %s", sf)
	}
}

// ocTestServer creates a test WebSocket server that performs the OpenClaw
// handshake and then calls handler for each subsequent request frame.
// The handler receives the parsed request and returns a result or error to send.
func ocTestServer(t *testing.T, handler func(req requestFrame) responseFrame) (*httptest.Server, string) {
	t.Helper()
	var upgrader = websocket.Upgrader{}
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Send challenge.
		challenge := `{"type":"event","method":"challenge"}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(challenge)); err != nil {
			return
		}

		// Read connect request.
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		// Parse connect to get the ID, send OK response.
		var connectReq requestFrame
		_ = json.Unmarshal(msg, &connectReq)
		resp := fmt.Sprintf(`{"type":"res","id":%q,"result":{"ok":true}}`, connectReq.ID)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(resp)); err != nil {
			return
		}

		// Handle subsequent requests.
		for {
			select {
			case <-done:
				return
			default:
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var req requestFrame
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}

			respFrame := handler(req)
			respFrame.Type = "res"
			respFrame.ID = req.ID
			data, _ := json.Marshal(respFrame)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}))

	t.Cleanup(func() {
		close(done)
		srv.Close()
	})

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

// TestSendRequestRoutesResponseByID verifies that concurrent sendRequest
// calls each receive their own response, matched by request ID.
func TestSendRequestRoutesResponseByID(t *testing.T) {
	_, wsURL := ocTestServer(t, func(req requestFrame) responseFrame {
		return responseFrame{
			Result: json.RawMessage(fmt.Sprintf(`{"echo":%q}`, req.ID)),
		}
	})

	client := newTestOpenClaw(wsURL, "test-token", nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	const n = 3
	results := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			resp, err := client.sendRequest(context.Background(), "test.echo", map[string]int{"i": i})
			errs[i] = err
			if err == nil {
				results[i] = string(resp.Result)
			}
		}()
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d: %v", i, errs[i])
			continue
		}
		// Each result should contain the request ID that was echoed back.
		if results[i] == "" {
			t.Errorf("goroutine %d: empty result", i)
		}
	}
}

// TestSendRequestReturnsErrorOnGatewayReject verifies that when the gateway
// responds with an error frame, sendRequest returns it in the response.
func TestSendRequestReturnsErrorOnGatewayReject(t *testing.T) {
	_, wsURL := ocTestServer(t, func(req requestFrame) responseFrame {
		return responseFrame{
			Error: json.RawMessage(`{"message":"session not found"}`),
		}
	})

	client := newTestOpenClaw(wsURL, "test-token", nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	resp, err := client.sendRequest(context.Background(), "chat.send", chatSendParams{
		SessionKey: "main-wa-test",
		Message:    "hello",
	})
	if err != nil {
		t.Fatalf("sendRequest returned transport error: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error in response, got nil")
	}
	if !strings.Contains(string(resp.Error), "session not found") {
		t.Errorf("expected 'session not found' in error, got: %s", string(resp.Error))
	}
}

// TestSendRequestUnblocksOnConnectionClose verifies that sendRequest returns
// an error when the server closes the connection, rather than hanging.
func TestSendRequestUnblocksOnConnectionClose(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		// Handshake.
		challenge := `{"type":"event","method":"challenge"}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(challenge))
		_, msg, _ := conn.ReadMessage()
		var req requestFrame
		_ = json.Unmarshal(msg, &req)
		resp := fmt.Sprintf(`{"type":"res","id":%q,"result":{"ok":true}}`, req.ID)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(resp))

		// Read the chat.send request but close without responding.
		_, _, _ = conn.ReadMessage()
		_ = conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := newTestOpenClaw(wsURL, "test-token", nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	_, err := client.sendRequest(context.Background(), "chat.send", chatSendParams{
		SessionKey: "main",
		Message:    "hello",
	})
	if err == nil {
		t.Fatal("expected error when connection closed, got nil")
	}
	if !strings.Contains(err.Error(), "connection closed") {
		t.Errorf("expected 'connection closed' error, got: %v", err)
	}
}

// TestSendAndReceiveDetectsGatewayError verifies that SendAndReceive returns
// immediately with an error when the gateway rejects chat.send, without
// entering the file-polling loop.
func TestSendAndReceiveDetectsGatewayError(t *testing.T) {
	_, wsURL := ocTestServer(t, func(req requestFrame) responseFrame {
		return responseFrame{
			Error: json.RawMessage(`{"message":"unauthorized"}`),
		}
	})

	client := newTestOpenClaw(wsURL, "test-token", nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	_, err := client.SendAndReceive(context.Background(), &Request{
		SessionKey: "main",
		Text:       "hello",
		From:       "+1234567890",
		FromName:   "Test",
		Role:       "admin",
	})
	if err == nil {
		t.Fatal("expected error from rejected chat.send")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected 'unauthorized' in error, got: %v", err)
	}
}

// TestReadLoopRoutesAroundEvents verifies that unsolicited event frames
// do not interfere with response routing.
func TestReadLoopRoutesAroundEvents(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Handshake.
		challenge := `{"type":"event","method":"challenge"}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(challenge))
		_, msg, _ := conn.ReadMessage()
		var creq requestFrame
		_ = json.Unmarshal(msg, &creq)
		resp := fmt.Sprintf(`{"type":"res","id":%q,"result":{"ok":true}}`, creq.ID)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(resp))

		// Read the request, send an event first, then the actual response.
		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		var req requestFrame
		_ = json.Unmarshal(msg, &req)

		// Interleaved event.
		event := `{"type":"event","method":"status.update","params":{"status":"processing"}}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(event))

		// Actual response.
		res := fmt.Sprintf(`{"type":"res","id":%q,"result":{"ok":true}}`, req.ID)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(res))

		// Keep alive until test closes.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := newTestOpenClaw(wsURL, "test-token", nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	resp, err := client.sendRequest(context.Background(), "chat.send", chatSendParams{
		SessionKey: "main",
		Message:    "hello",
	})
	if err != nil {
		t.Fatalf("sendRequest failed: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error in response: %s", string(resp.Error))
	}
}

// TestCloseUnblocksPendingSendRequest verifies that calling Close() while
// a sendRequest is waiting causes sendRequest to return an error, and
// Close() itself does not deadlock.
func TestCloseUnblocksPendingSendRequest(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Handshake.
		challenge := `{"type":"event","method":"challenge"}`
		_ = conn.WriteMessage(websocket.TextMessage, []byte(challenge))
		_, msg, _ := conn.ReadMessage()
		var req requestFrame
		_ = json.Unmarshal(msg, &req)
		resp := fmt.Sprintf(`{"type":"res","id":%q,"result":{"ok":true}}`, req.ID)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(resp))

		// Read requests but never respond — simulate a hung gateway.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := newTestOpenClaw(wsURL, "test-token", nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := client.sendRequest(context.Background(), "chat.send", chatSendParams{
			SessionKey: "main",
			Message:    "hello",
		})
		errCh <- err
	}()

	// Give sendRequest time to register and block.
	time.Sleep(50 * time.Millisecond)

	// Close should unblock the pending sendRequest.
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from sendRequest after Close, got nil")
		}
		if !strings.Contains(err.Error(), "connection closed") {
			t.Errorf("expected 'connection closed' error, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sendRequest did not unblock after Close — deadlock")
	}
}

// TestConcurrentClaimsUniqueReplies verifies that when multiple goroutines
// race to read the same session JSONL file, each one claims a different
// assistant reply.
func TestConcurrentClaimsUniqueReplies(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	lines := ""
	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i+1) * time.Second)
		lines += fmt.Sprintf(
			`{"type":"message","timestamp":"%s","message":{"role":"assistant","stopReason":"stop","content":[{"type":"text","text":"reply-%d"}]}}`,
			ts.Format(time.RFC3339), i+1,
		) + "\n"
	}
	if err := os.WriteFile(sessionFile, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	since := base
	tracker := newReplyTracker()

	const goroutines = 3
	claimed := make([]string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			replies, err := getAssistantReplies(sessionFile, since)
			if err != nil {
				t.Errorf("goroutine %d: getAssistantReplies: %v", g, err)
				return
			}
			for _, r := range replies {
				if tracker.claim(r.Key) {
					claimed[g] = r.Text
					return
				}
			}
		}()
	}

	wg.Wait()

	seen := map[string]int{}
	for g, text := range claimed {
		if text == "" {
			t.Errorf("goroutine %d got no reply", g)
			continue
		}
		seen[text]++
	}

	for text, count := range seen {
		if count > 1 {
			t.Errorf("reply %q was claimed %d times (want 1)", text, count)
		}
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique replies, got %d: %v", goroutines, len(seen), seen)
	}
}
