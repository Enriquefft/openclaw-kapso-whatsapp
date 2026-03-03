package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RequestFrame is an outgoing request to the OpenClaw gateway.
type RequestFrame struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// ResponseFrame is an incoming response/event from the gateway.
type ResponseFrame struct {
	Type   string          `json:"type"`
	ID     string          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// ConnectParams is the params for the connect request.
type ConnectParams struct {
	MinProtocol int         `json:"minProtocol"`
	MaxProtocol int         `json:"maxProtocol"`
	Client      ClientInfo  `json:"client"`
	Auth        AuthInfo    `json:"auth"`
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

// ClientInfo identifies this client to the gateway.
type ClientInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Mode        string `json:"mode"`
}

// AuthInfo contains authentication credentials.
type AuthInfo struct {
	Token string `json:"token"`
}

// ChatSendParams is the payload for chat.send requests to the OpenClaw gateway.
// sessionKey "main" targets the agent's primary session.
type ChatSendParams struct {
	SessionKey     string `json:"sessionKey"`
	Message        string `json:"message"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// Client manages a WebSocket connection to the OpenClaw gateway.
type Client struct {
	url    string
	token  string
	signer Signer
	conn   *websocket.Conn
	mu     sync.Mutex
	seq    int
}

// NewClient creates a new gateway WebSocket client. The signer provides device
// identity for the connect handshake; pass nil to connect without device identity.
func NewClient(url, token string, signer Signer) *Client {
	return &Client{
		url:    url,
		token:  token,
		signer: signer,
	}
}

func (c *Client) nextID() string {
	c.seq++
	return fmt.Sprintf("kapso-%d", c.seq)
}

// Connect establishes the WebSocket connection and completes the challenge-response auth.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	c.conn = conn

	// Read the challenge from the gateway.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("read challenge: %w", err)
	}

	log.Printf("received challenge from gateway: %s", string(msg))

	// Parse challenge to extract nonce for device signing.
	var challenge struct {
		Params struct {
			Nonce string `json:"nonce"`
		} `json:"params"`
	}
	if err := json.Unmarshal(msg, &challenge); err != nil {
		log.Printf("warning: failed to parse challenge frame: %v", err)
	}

	// Build device identity if a signer is configured.
	var deviceInfo *DeviceInfo
	if c.signer != nil && challenge.Params.Nonce != "" {
		sig, signedAt, err := c.signer.Sign(challenge.Params.Nonce)
		if err != nil {
			_ = conn.Close()
			c.conn = nil
			return fmt.Errorf("sign challenge nonce: %w", err)
		}
		deviceInfo = &DeviceInfo{
			ID:        c.signer.DeviceID(),
			PublicKey: c.signer.PublicKeyBase64(),
			Signature: sig,
			SignedAt:  signedAt,
			Nonce:     challenge.Params.Nonce,
		}
	}

	// Send connect request.
	connectReq := RequestFrame{
		Type:   "req",
		ID:     c.nextID(),
		Method: "connect",
		Params: ConnectParams{
			MinProtocol: 3,
			MaxProtocol: 3,
			Client: ClientInfo{
				ID:          "gateway-client",
				DisplayName: "Kapso WhatsApp Bridge",
				Version:     "0.2.0",
				Platform:    "linux",
				Mode:        "backend",
			},
			Auth: AuthInfo{
				Token: c.token,
			},
			Device: deviceInfo,
			Role:   "operator",
			Scopes: []string{"operator.read", "operator.write"},
		},
	}

	data, err := json.Marshal(connectReq)
	if err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("marshal connect request: %w", err)
	}

	log.Printf("sending connect request")

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("send connect: %w", err)
	}

	// Wait for response.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("read connect response: %w", err)
	}

	log.Printf("connect response: %s", string(msg))

	var resp ResponseFrame
	if err := json.Unmarshal(msg, &resp); err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("parse connect response: %w", err)
	}

	if resp.Error != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("connect rejected: %s", string(resp.Error))
	}

	// Clear deadline for normal operation.
	_ = conn.SetReadDeadline(time.Time{})

	// Drain unsolicited gateway events in the background so the socket
	// buffer never fills up and write operations don't stall.
	go c.drain()

	log.Printf("authenticated with gateway at %s", c.url)
	return nil
}

// drain reads and discards all incoming frames from the gateway. It runs as a
// background goroutine after Connect succeeds and exits when the connection is
// closed.
func (c *Client) drain() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		log.Printf("gateway event: %s", string(msg))
	}
}

// Send submits a WhatsApp message to the OpenClaw gateway via chat.send.
// The message is delivered to the agent's "main" session. The sender's phone
// number and display name are embedded in the message text so the agent knows
// who to reply to.
func (c *Client) Send(sessionKey, idempotencyKey, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to gateway")
	}

	req := RequestFrame{
		Type:   "req",
		ID:     c.nextID(),
		Method: "chat.send",
		Params: ChatSendParams{
			SessionKey:     sessionKey,
			Message:        message,
			IdempotencyKey: idempotencyKey,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	return nil
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
