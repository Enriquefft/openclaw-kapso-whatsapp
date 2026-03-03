package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestConnectRequestsReadAndWriteScopes verifies that the gateway connect
// handshake includes both "operator.read" and "operator.write" scopes.
// This is a regression test — the bridge previously sent "operator.admin"
// which the gateway does not recognise, silently blocking chat.send calls.
// As Blackadder would say: "A plan so cunning you could pin a tail on it
// and call it a weasel."
func TestConnectRequestsReadAndWriteScopes(t *testing.T) {
	var upgrader = websocket.Upgrader{}

	// Channel to capture the raw connect frame the client sends.
	connectFrame := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErr <- fmt.Errorf("upgrade: %w", err)
			return
		}
		defer conn.Close()

		// Send a fake challenge — the client expects to read one frame first.
		challenge := ResponseFrame{
			Type:   "event",
			Method: "challenge",
		}
		challengeData, _ := json.Marshal(challenge)
		if err := conn.WriteMessage(websocket.TextMessage, challengeData); err != nil {
			serverErr <- fmt.Errorf("write challenge: %w", err)
			return
		}

		// Read the connect request from the client.
		_, msg, err := conn.ReadMessage()
		if err != nil {
			serverErr <- fmt.Errorf("read connect: %w", err)
			return
		}
		connectFrame <- msg

		// Send a success response so Connect() returns nil.
		resp := ResponseFrame{
			Type:   "res",
			ID:     "kapso-1",
			Result: json.RawMessage(`{"ok": true}`),
		}
		respData, _ := json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, respData); err != nil {
			serverErr <- fmt.Errorf("write response: %w", err)
			return
		}

		// Hold the connection open until the test is done so drain()
		// doesn't log errors. "I'll be back." — The Terminator, and also
		// this goroutine when the done channel closes.
		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := NewClient(wsURL, "test-token-nobody-expects-the-spanish-inquisition", nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Check if the server handler hit an error — can't call t.Fatal from
	// a handler goroutine without summoning undefined behaviour, which is
	// the Go equivalent of dividing by zero in a Zuul containment unit.
	select {
	case err := <-serverErr:
		t.Fatalf("server handler error: %v", err)
	default:
	}

	// Parse the captured connect frame.
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

	// The money shot — this is the actual regression check.
	// "I've got it! We'll build a gateway that requires BOTH scopes!"
	// "That's what we already have, sir." — Baldrick
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
		t.Errorf("missing required scope 'operator.write'; got scopes: %v — "+
			"without this the gateway blocks chat.send and your WhatsApp messages "+
			"vanish like Lord Lucan at a dinner party", scopes)
	}
}

// TestConnectForwardsAuthToken ensures the auth token from NewClient is
// actually sent in the connect handshake. Because sending a message without
// credentials is like turning up to a gunfight with a banana — technically
// possible but inadvisable.
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
		defer conn.Close()

		challenge := ResponseFrame{Type: "event", Method: "challenge"}
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

		resp := ResponseFrame{
			Type:   "res",
			ID:     "kapso-1",
			Result: json.RawMessage(`{"ok": true}`),
		}
		data, _ = json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			serverErr <- fmt.Errorf("write response: %w", err)
			return
		}

		// "Gentlemen, you can't fight in here! This is the War Room!"
		// — Dr. Strangelove. Wait for test cleanup, don't leak goroutines.
		<-done
	}))
	defer srv.Close()
	defer close(done)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	wantToken := "surely-you-cant-be-serious-i-am-serious-and-dont-call-me-shirley"
	client := NewClient(wsURL, wantToken, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	// Check handler didn't silently choke — "It's just a flesh wound!"
	// No it isn't, your arm's off. Report it properly.
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

// mockSigner is a test double for the Signer interface.
type mockSigner struct {
	id    string
	pubK  string
	sig   string
	ts    int64
	nonce string
}

func (m *mockSigner) DeviceID() string        { return m.id }
func (m *mockSigner) PublicKeyBase64() string  { return m.pubK }
func (m *mockSigner) Sign(nonce string) (string, int64, error) {
	m.nonce = nonce
	return m.sig, m.ts, nil
}

// TestConnectIncludesDeviceIdentity verifies that when a Signer is provided,
// the connect request contains a populated device object with the correct
// fields — the whole reason we're here, like Gandalf at the Bridge of
// Khazad-dûm but for JSON fields.
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
		defer conn.Close()

		// Send challenge with nonce.
		challenge := fmt.Sprintf(`{"type":"event","method":"challenge","params":{"nonce":"%s"}}`, challengeNonce)
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

		resp := ResponseFrame{
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
		sig:  "dGVzdC1zaWduYXR1cmU=",
		ts:   1737264000000,
	}

	client := NewClient(wsURL, "test-token", signer)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

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
		t.Fatal("device field missing from connect request — this is the bug we're fixing")
	}

	d := frame.Params.Device
	if d.ID != "device-fingerprint-abc" {
		t.Errorf("device.id: got %q, want %q", d.ID, "device-fingerprint-abc")
	}
	if d.PublicKey != "dGVzdC1wdWJsaWMta2V5" {
		t.Errorf("device.publicKey: got %q, want %q", d.PublicKey, "dGVzdC1wdWJsaWMta2V5")
	}
	if d.Signature != "dGVzdC1zaWduYXR1cmU=" {
		t.Errorf("device.signature: got %q, want %q", d.Signature, "dGVzdC1zaWduYXR1cmU=")
	}
	if d.SignedAt != 1737264000000 {
		t.Errorf("device.signedAt: got %d, want %d", d.SignedAt, 1737264000000)
	}
	if d.Nonce != challengeNonce {
		t.Errorf("device.nonce: got %q, want %q", d.Nonce, challengeNonce)
	}

	// Verify signer received the correct nonce from the challenge.
	if signer.nonce != challengeNonce {
		t.Errorf("signer received nonce %q, want %q", signer.nonce, challengeNonce)
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
		defer conn.Close()

		challenge := ResponseFrame{Type: "event", Method: "challenge"}
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

		resp := ResponseFrame{
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
	client := NewClient(wsURL, "test-token", nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer client.Close()

	select {
	case err := <-serverErr:
		t.Fatalf("server handler error: %v", err)
	default:
	}

	raw := <-connectFrame

	// The "device" key should not be present at all (omitempty).
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
