package webhook

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/gateway"
	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

// Handler processes incoming webhook payloads and forwards them to the gateway.
type Handler struct {
	gw *gateway.Client
}

// NewHandler creates a webhook handler.
func NewHandler(gw *gateway.Client) *Handler {
	return &Handler{gw: gw}
}

// HandlePayload processes a verified webhook payload.
func (h *Handler) HandlePayload(data []byte) error {
	var payload kapso.WebhookPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal webhook payload: %w", err)
	}

	if payload.Object != "whatsapp_business_account" {
		log.Printf("ignoring non-whatsapp object: %s", payload.Object)
		return nil
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			h.processMessages(change.Value)
		}
	}

	return nil
}

func (h *Handler) processMessages(value kapso.ChangeValue) {
	contactNames := make(map[string]string)
	for _, c := range value.Contacts {
		contactNames[c.WaID] = c.Profile.Name
	}

	for _, msg := range value.Messages {
		if msg.Type != "text" || msg.Text == nil {
			log.Printf("skipping non-text message (type=%s) from %s", msg.Type, msg.From)
			continue
		}

		name := contactNames[msg.From]
		gwMsg := gateway.GatewayMessage{
			Type:    "message",
			Channel: "whatsapp",
			From:    msg.From,
			Name:    name,
			Text:    msg.Text.Body,
		}

		log.Printf("forwarding message from %s (%s): %s", msg.From, name, truncate(msg.Text.Body, 50))

		if err := h.gw.Send(gwMsg); err != nil {
			log.Printf("error forwarding to gateway: %v", err)
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
