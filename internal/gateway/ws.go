package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// EventFrame is the OpenClaw gateway message envelope.
type EventFrame struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Seq     int             `json:"seq,omitempty"`
	ID      string          `json:"id,omitempty"`
}

// ChallengePayload is sent by the gateway on connect.
type ChallengePayload struct {
	Nonce string `json:"nonce"`
}

// ConnectPayload is sent by the client to authenticate.
type ConnectPayload struct {
	MinProtocol int          `json:"minProtocol"`
	MaxProtocol int          `json:"maxProtocol"`
	Client      ClientInfo   `json:"client"`
	Auth        AuthInfo     `json:"auth"`
	Role        string       `json:"role"`
	Scopes      []string     `json:"scopes"`
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

// GatewayMessage is the message format sent to the OpenClaw gateway.
type GatewayMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	From    string `json:"from"`
	Name    string `json:"name,omitempty"`
	Text    string `json:"text"`
}

// Client manages a WebSocket connection to the OpenClaw gateway.
type Client struct {
	url   string
	token string
	conn  *websocket.Conn
	mu    sync.Mutex
}

// NewClient creates a new gateway WebSocket client.
func NewClient(url, token string) *Client {
	return &Client{
		url:   url,
		token: token,
	}
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
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("read challenge: %w", err)
	}

	var frame EventFrame
	if err := json.Unmarshal(msg, &frame); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("parse challenge frame: %w", err)
	}

	if frame.Event != "connect.challenge" {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("expected connect.challenge, got %s", frame.Event)
	}

	log.Printf("received challenge from gateway")

	// Send connect response with auth token.
	connectPayload := ConnectPayload{
		MinProtocol: 1,
		MaxProtocol: 1,
		Client: ClientInfo{
			ID:          "kapso-whatsapp",
			DisplayName: "Kapso WhatsApp Bridge",
			Version:     "0.2.0",
			Platform:    "linux",
			Mode:        "backend",
		},
		Auth: AuthInfo{
			Token: c.token,
		},
		Role:   "operator",
		Scopes: []string{"operator.admin"},
	}

	payloadBytes, err := json.Marshal(connectPayload)
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("marshal connect payload: %w", err)
	}

	connectFrame := EventFrame{
		Event:   "connect",
		Payload: payloadBytes,
	}

	frameBytes, err := json.Marshal(connectFrame)
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("marshal connect frame: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, frameBytes); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("send connect: %w", err)
	}

	// Wait for hello.ok response.
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("read hello response: %w", err)
	}

	var helloFrame EventFrame
	if err := json.Unmarshal(msg, &helloFrame); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("parse hello frame: %w", err)
	}

	if helloFrame.Event != "hello.ok" {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("auth failed: got %s (payload: %s)", helloFrame.Event, string(helloFrame.Payload))
	}

	// Clear deadline for normal operation.
	conn.SetReadDeadline(time.Time{})

	log.Printf("authenticated with gateway at %s", c.url)
	return nil
}

// Send sends a message to the gateway using the EventFrame protocol.
func (c *Client) Send(msg GatewayMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to gateway")
	}

	payloadBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	frame := EventFrame{
		Event:   "message.receive",
		Payload: payloadBytes,
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
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
