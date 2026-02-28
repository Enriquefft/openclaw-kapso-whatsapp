package kapso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const baseURL = "https://api.kapso.ai/meta/whatsapp/v24.0"

// Client sends messages via the Kapso WhatsApp API.
type Client struct {
	APIKey        string
	PhoneNumberID string
	HTTPClient    *http.Client
}

// NewClient creates a Kapso API client.
func NewClient(apiKey, phoneNumberID string) *Client {
	return &Client{
		APIKey:        apiKey,
		PhoneNumberID: phoneNumberID,
		HTTPClient:    http.DefaultClient,
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

	url := fmt.Sprintf("%s/%s/messages", baseURL, c.PhoneNumberID)
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
	defer resp.Body.Close()

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
