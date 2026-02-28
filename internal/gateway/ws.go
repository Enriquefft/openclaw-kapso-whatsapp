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
	MinProtocol int        `json:"minProtocol"`
	MaxProtocol int        `json:"maxProtocol"`
	Client      ClientInfo `json:"client"`
	Auth        AuthInfo   `json:"auth"`
	Role        string     `json:"role"`
	Scopes      []string   `json:"scopes"`
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

// GatewayMessage is the message payload sent to the OpenClaw gateway.
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
	seq   int
}

// NewClient creates a new gateway WebSocket client.
func NewClient(url, token string) *Client {
	return &Client{
		url:   url,
		token: token,
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
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("read challenge: %w", err)
	}

	log.Printf("received challenge from gateway: %s", string(msg))

	// Send connect request.
	connectReq := RequestFrame{
		Type:   "req",
		ID:     c.nextID(),
		Method: "connect",
		Params: ConnectParams{
			MinProtocol: 1,
			MaxProtocol: 1,
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
			Role:   "operator",
			Scopes: []string{"operator.admin"},
		},
	}

	data, err := json.Marshal(connectReq)
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("marshal connect request: %w", err)
	}

	log.Printf("sending connect request")

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("send connect: %w", err)
	}

	// Wait for response.
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("read connect response: %w", err)
	}

	log.Printf("connect response: %s", string(msg))

	var resp ResponseFrame
	if err := json.Unmarshal(msg, &resp); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("parse connect response: %w", err)
	}

	if resp.Error != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("connect rejected: %s", string(resp.Error))
	}

	// Clear deadline for normal operation.
	conn.SetReadDeadline(time.Time{})

	log.Printf("authenticated with gateway at %s", c.url)
	return nil
}

// Send sends a message to the gateway.
func (c *Client) Send(msg GatewayMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to gateway")
	}

	req := RequestFrame{
		Type:   "req",
		ID:     c.nextID(),
		Method: "message.receive",
		Params: msg,
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
