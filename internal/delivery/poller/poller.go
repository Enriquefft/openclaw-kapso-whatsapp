package poller

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/delivery"
	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/kapso"
)

// Poller implements delivery.Source by polling the Kapso list-messages API.
type Poller struct {
	Client    *kapso.Client
	Interval  time.Duration
	StateDir  string
	StateFile string
}

// Run polls the Kapso API on a ticker and emits events for each new inbound
// message. It returns when ctx is cancelled.
func (p *Poller) Run(ctx context.Context, out chan<- delivery.Event) error {
	os.MkdirAll(p.StateDir, 0o700)

	lastPoll := loadState(p.StateFile)
	if lastPoll.IsZero() {
		lastPoll = time.Now().UTC()
		saveState(p.StateFile, lastPoll)
		log.Printf("first run, starting from %s", lastPoll.Format(time.RFC3339))
	}

	// Poll immediately, then on interval.
	p.poll(&lastPoll, out)

	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.poll(&lastPoll, out)
		}
	}
}

func (p *Poller) poll(lastPoll *time.Time, out chan<- delivery.Event) {
	since := lastPoll.Format(time.RFC3339)

	resp, err := p.Client.ListMessages(kapso.ListMessagesParams{
		Direction: "inbound",
		Since:     since,
		Limit:     100,
	})
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	if len(resp.Data) == 0 {
		return
	}

	var newest time.Time
	forwarded := 0

	for _, msg := range resp.Data {
		text, ok := delivery.ExtractText(msg.Message, p.Client)
		if !ok {
			continue
		}

		name := ""
		if msg.Kapso != nil {
			name = msg.Kapso.ContactName
		}

		out <- delivery.Event{
			ID:   msg.ID,
			From: msg.From,
			Name: name,
			Text: text,
		}
		forwarded++

		msgTime := parseTimestamp(msg.Timestamp)
		if !msgTime.IsZero() && msgTime.After(newest) {
			newest = msgTime
		}
	}

	if forwarded > 0 {
		log.Printf("forwarded %d message(s)", forwarded)
	}

	if !newest.IsZero() {
		*lastPoll = newest.Add(time.Second)
		saveState(p.StateFile, *lastPoll)
	}
}

func parseTimestamp(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return time.Unix(n, 0).UTC()
	}
	return time.Time{}
}

func loadState(path string) time.Time {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}
	return t
}

func saveState(path string, t time.Time) {
	os.WriteFile(path, []byte(t.Format(time.RFC3339)), 0o600)
}
