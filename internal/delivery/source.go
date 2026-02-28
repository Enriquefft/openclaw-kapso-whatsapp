package delivery

import (
	"context"
)

// Event represents a single inbound message ready for the gateway.
type Event struct {
	ID   string // Kapso message ID (idempotency key)
	From string // sender phone
	Name string // contact display name
	Text string // extracted, gateway-ready text
}

// Source produces inbound message events from a delivery channel (poller, webhook, etc.).
type Source interface {
	Run(ctx context.Context, out chan<- Event) error
}
