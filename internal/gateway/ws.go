package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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

// Connect establishes the WebSocket connection.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	header := make(map[string][]string)
	if c.token != "" {
		header["Authorization"] = []string{"Bearer " + c.token}
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, header)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}

	c.conn = conn
	log.Printf("connected to gateway at %s", c.url)
	return nil
}

// Send sends a message to the gateway.
func (c *Client) Send(msg GatewayMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected to gateway")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
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
