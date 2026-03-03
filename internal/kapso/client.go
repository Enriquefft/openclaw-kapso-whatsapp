package kapso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.kapso.ai/meta/whatsapp/v24.0"

// Client sends messages via the Kapso WhatsApp API.
type Client struct {
	APIKey        string
	PhoneNumberID string
	HTTPClient    *http.Client
	BaseURL       string // if empty, uses the default Kapso API URL
}

// getBaseURL returns the configured base URL or the default.
func (c *Client) getBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return baseURL
}

// NewClient creates a Kapso API client with a 30-second HTTP timeout.
func NewClient(apiKey, phoneNumberID string) *Client {
	return &Client{
		APIKey:        apiKey,
		PhoneNumberID: phoneNumberID,
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// SendText sends a text message to the given phone number.
func (c *Client) SendText(to, text string) (*SendMessageResponse, error) {
	req := SendMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "text",
		Text:             TextContent{Body: text},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", c.getBaseURL(), c.PhoneNumberID)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("kapso API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result SendMessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// MarkRead marks a message as read. This sends blue checkmarks to the sender.
func (c *Client) MarkRead(messageID string) error {
	return c.markRead(messageID, nil)
}

// MarkReadWithTyping marks a message as read and shows a typing indicator.
func (c *Client) MarkReadWithTyping(messageID string) error {
	return c.markRead(messageID, &TypingIndicator{Type: "text"})
}

// markRead posts a mark-as-read request, optionally with a typing indicator.
func (c *Client) markRead(messageID string, typing *TypingIndicator) error {
	req := MarkReadRequest{
		MessagingProduct: "whatsapp",
		Status:           "read",
		MessageID:        messageID,
		TypingIndicator:  typing,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", c.getBaseURL(), c.PhoneNumberID)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("mark read error (status %d)", resp.StatusCode)
	}

	return nil
}

// DownloadMedia downloads raw audio bytes from the given URL, enforcing a
// maximum response size. The maxBytes limit is applied via io.LimitReader with
// a +1 sentinel: if the server sends more than maxBytes, an error is returned.
func (c *Client) DownloadMedia(url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errBody := make([]byte, 512)
		n, _ := resp.Body.Read(errBody)
		return nil, fmt.Errorf("media download error (status %d): %s", resp.StatusCode, string(errBody[:n]))
	}

	// Read up to maxBytes+1 to detect responses that exceed the limit.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read media body: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("media response exceeds size limit (%d bytes)", maxBytes)
	}

	return data, nil
}
