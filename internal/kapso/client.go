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

// SendTypingIndicator sends a typing indicator to the given phone number to
// show that a response is being prepared.
func (c *Client) SendTypingIndicator(to string) error {
	req := TypingIndicatorRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "typing",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", baseURL, c.PhoneNumberID)
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
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("typing indicator error (status %d)", resp.StatusCode)
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
	defer resp.Body.Close()

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

// GetMediaURL retrieves the download URL for a media attachment by its ID.
func (c *Client) GetMediaURL(mediaID string) (*MediaResponse, error) {
	url := fmt.Sprintf("%s/%s", baseURL, mediaID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get media URL: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kapso API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result MediaResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
