package kapso

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ListMessagesParams are query parameters for listing messages.
type ListMessagesParams struct {
	Direction string // "inbound" or "outbound"
	Since     string // ISO 8601 timestamp
	Limit     int
	After     string // pagination cursor
}

// ListMessagesResponse is the response from the list messages API.
type ListMessagesResponse struct {
	Data   []InboundMessage `json:"data"`
	Paging *Paging          `json:"paging,omitempty"`
}

// InboundMessage represents a message from the list API.
// It embeds Message to promote shared fields (ID, From, Type, Text, Image, etc.).
type InboundMessage struct {
	Message            // promotes ID, From, Type, Text, Image, etc.
	To      string     `json:"to,omitempty"`
	Kapso   *KapsoMeta `json:"kapso,omitempty"`
}

// KapsoMeta contains Kapso-enhanced metadata.
type KapsoMeta struct {
	Direction   string `json:"direction"`
	Status      string `json:"status"`
	ContactName string `json:"contact_name"`
}

// Paging contains cursor-based pagination info.
type Paging struct {
	Cursors struct {
		After  string `json:"after"`
		Before string `json:"before"`
	} `json:"cursors"`
}

// ListMessages fetches messages from the Kapso API.
func (c *Client) ListMessages(params ListMessagesParams) (*ListMessagesResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/messages", baseURL, c.PhoneNumberID))
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	q := u.Query()
	if params.Direction != "" {
		q.Set("direction", params.Direction)
	}
	if params.Since != "" {
		q.Set("since", params.Since)
	}
	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.After != "" {
		q.Set("after", params.After)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kapso API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result ListMessagesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
