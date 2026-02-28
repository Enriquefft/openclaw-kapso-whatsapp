package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

// TestConcurrentRelayClaimsUniqueReplies verifies that when multiple relay
// goroutines race to read the same session JSONL file, each one claims a
// different assistant reply — no duplicates, no missed replies.
func TestConcurrentRelayClaimsUniqueReplies(t *testing.T) {
	// Create a temp session JSONL with 3 assistant replies, all timestamped
	// after our "since" threshold.
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Three assistant stop-messages, each 1 second apart.
	lines := ""
	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i+1) * time.Second)
		lines += fmt.Sprintf(
			`{"type":"message","timestamp":"%s","message":{"role":"assistant","stopReason":"stop","content":[{"type":"text","text":"reply-%d"}]}}`,
			ts.Format(time.RFC3339), i+1,
		) + "\n"
	}
	if err := os.WriteFile(sessionFile, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	since := base // all replies are after this time
	tracker := newRelayTracker()

	// Simulate 3 concurrent relay goroutines racing to claim replies.
	const goroutines = 3
	claimed := make([]string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			replies, err := getAssistantReplies(sessionFile, since)
			if err != nil {
				t.Errorf("goroutine %d: getAssistantReplies: %v", g, err)
				return
			}
			for _, r := range replies {
				if tracker.claim(r.Key) {
					claimed[g] = r.Text
					return
				}
			}
		}()
	}

	wg.Wait()

	// Collect results: every goroutine should have claimed exactly one
	// unique reply.
	seen := map[string]int{}
	for g, text := range claimed {
		if text == "" {
			t.Errorf("goroutine %d got no reply", g)
			continue
		}
		seen[text]++
	}

	for text, count := range seen {
		if count > 1 {
			t.Errorf("reply %q was claimed %d times (want 1)", text, count)
		}
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique replies, got %d: %v", goroutines, len(seen), seen)
	}
}

func TestExtractMessageText_Text(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m1",
		Type: "text",
		From: "+1234567890",
		Text: &kapso.TextContent{Body: "hello world"},
	}
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for text message")
	}
	if text != "hello world" {
		t.Fatalf("got %q, want %q", text, "hello world")
	}
}

func TestExtractMessageText_TextNilBody(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m2",
		Type: "text",
		From: "+1234567890",
	}
	_, ok := extractMessageText(msg, nil)
	if ok {
		t.Fatal("expected ok=false for text message with nil Text")
	}
}

func TestExtractMessageText_Image(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m3",
		Type: "image",
		From: "+1234567890",
		Image: &kapso.ImageContent{
			ID:       "media-123",
			MimeType: "image/jpeg",
			Caption:  "sunset photo",
		},
	}
	// Pass nil client to skip media URL retrieval.
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for image message")
	}
	if !strings.Contains(text, "[image]") {
		t.Errorf("expected [image] tag in %q", text)
	}
	if !strings.Contains(text, "sunset photo") {
		t.Errorf("expected caption in %q", text)
	}
	if !strings.Contains(text, "image/jpeg") {
		t.Errorf("expected mime type in %q", text)
	}
}

func TestExtractMessageText_ImageWithMediaURL(t *testing.T) {
	// Set up a mock server that returns a media URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(kapso.MediaResponse{
			URL:      "https://example.com/media/photo.jpg",
			MimeType: "image/jpeg",
			ID:       "media-123",
		})
	}))
	defer srv.Close()

	client := &kapso.Client{
		APIKey:        "test-key",
		PhoneNumberID: "12345",
		HTTPClient:    srv.Client(),
	}
	// Override the base URL by using a custom transport that rewrites URLs.
	client.HTTPClient = &http.Client{
		Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport},
	}

	msg := kapso.InboundMessage{
		ID:   "m3b",
		Type: "image",
		From: "+1234567890",
		Image: &kapso.ImageContent{
			ID:       "media-123",
			MimeType: "image/jpeg",
			Caption:  "sunset",
		},
	}
	text, ok := extractMessageText(msg, client)
	if !ok {
		t.Fatal("expected ok=true for image message")
	}
	if !strings.Contains(text, "https://example.com/media/photo.jpg") {
		t.Errorf("expected media URL in %q", text)
	}
}

func TestExtractMessageText_Document(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m4",
		Type: "document",
		From: "+1234567890",
		Document: &kapso.DocumentContent{
			ID:       "media-456",
			MimeType: "application/pdf",
			Filename: "report.pdf",
		},
	}
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for document message")
	}
	if !strings.Contains(text, "[document]") {
		t.Errorf("expected [document] tag in %q", text)
	}
	if !strings.Contains(text, "report.pdf") {
		t.Errorf("expected filename in %q", text)
	}
}

func TestExtractMessageText_DocumentCaptionFallback(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m4b",
		Type: "document",
		From: "+1234567890",
		Document: &kapso.DocumentContent{
			ID:       "media-456",
			MimeType: "application/pdf",
			Caption:  "my report",
		},
	}
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(text, "my report") {
		t.Errorf("expected caption fallback in %q", text)
	}
}

func TestExtractMessageText_Audio(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m5",
		Type: "audio",
		From: "+1234567890",
		Audio: &kapso.AudioContent{
			ID:       "media-789",
			MimeType: "audio/ogg",
		},
	}
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for audio message")
	}
	if !strings.Contains(text, "[audio]") {
		t.Errorf("expected [audio] tag in %q", text)
	}
	if !strings.Contains(text, "audio/ogg") {
		t.Errorf("expected mime type in %q", text)
	}
}

func TestExtractMessageText_Video(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m6",
		Type: "video",
		From: "+1234567890",
		Video: &kapso.VideoContent{
			ID:       "media-v1",
			MimeType: "video/mp4",
			Caption:  "funny clip",
		},
	}
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for video message")
	}
	if !strings.Contains(text, "[video]") {
		t.Errorf("expected [video] tag in %q", text)
	}
	if !strings.Contains(text, "funny clip") {
		t.Errorf("expected caption in %q", text)
	}
}

func TestExtractMessageText_Location(t *testing.T) {
	msg := kapso.InboundMessage{
		ID:   "m7",
		Type: "location",
		From: "+1234567890",
		Location: &kapso.LocationContent{
			Latitude:  -12.046374,
			Longitude: -77.042793,
			Name:      "Lima",
			Address:   "Peru",
		},
	}
	text, ok := extractMessageText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for location message")
	}
	if !strings.Contains(text, "[location]") {
		t.Errorf("expected [location] tag in %q", text)
	}
	if !strings.Contains(text, "Lima") {
		t.Errorf("expected name in %q", text)
	}
	if !strings.Contains(text, "-12.046374") {
		t.Errorf("expected latitude in %q", text)
	}
}

func TestExtractMessageText_UnsupportedType(t *testing.T) {
	// Create a mock server to capture the unsupported-type notification.
	// Use a channel to synchronize between the HTTP handler goroutine and
	// the test goroutine, avoiding a data race on sentTo/sentBody.
	type capture struct {
		to, body string
	}
	ch := make(chan capture, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req kapso.SendMessageRequest
		json.NewDecoder(r.Body).Decode(&req)
		ch <- capture{to: req.To, body: req.Text.Body}
		json.NewEncoder(w).Encode(kapso.SendMessageResponse{})
	}))
	defer srv.Close()

	client := &kapso.Client{
		APIKey:        "test-key",
		PhoneNumberID: "12345",
		HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
	}

	msg := kapso.InboundMessage{
		ID:   "m8",
		Type: "sticker",
		From: "+1234567890",
		Sticker: &kapso.StickerContent{
			ID:       "stk-1",
			MimeType: "image/webp",
		},
	}
	_, ok := extractMessageText(msg, client)
	if ok {
		t.Fatal("expected ok=false for unsupported sticker type")
	}

	// Wait for the background goroutine spawned by extractMessageText to
	// send the unsupported-type notification.
	got := <-ch
	if got.to != "+1234567890" {
		t.Errorf("notification sent to %q, want %q", got.to, "+1234567890")
	}
	if !strings.Contains(got.body, "sticker") {
		t.Errorf("notification body %q should mention sticker", got.body)
	}
}

func TestExtractMessageText_NilMediaContent(t *testing.T) {
	// Each media type with nil content struct should return false.
	for _, typ := range []string{"image", "document", "audio", "video", "location"} {
		msg := kapso.InboundMessage{
			ID:   "nil-" + typ,
			Type: typ,
			From: "+1234567890",
		}
		_, ok := extractMessageText(msg, nil)
		if ok {
			t.Errorf("expected ok=false for %s with nil content", typ)
		}
	}
}

func TestFormatMediaMessage_AllParts(t *testing.T) {
	text := formatMediaMessage("image", "my photo", "image/png", "", nil)
	want := "[image] my photo (image/png)"
	if text != want {
		t.Fatalf("got %q, want %q", text, want)
	}
}

func TestFormatMediaMessage_NoLabel(t *testing.T) {
	text := formatMediaMessage("audio", "", "audio/ogg", "", nil)
	want := "[audio] (audio/ogg)"
	if text != want {
		t.Fatalf("got %q, want %q", text, want)
	}
}

func TestFormatLocationMessage(t *testing.T) {
	loc := &kapso.LocationContent{
		Latitude:  40.714268,
		Longitude: -74.005974,
		Name:      "New York",
		Address:   "NY, USA",
	}
	text := formatLocationMessage(loc)
	if !strings.HasPrefix(text, "[location]") {
		t.Errorf("expected [location] prefix in %q", text)
	}
	if !strings.Contains(text, "New York") {
		t.Errorf("expected name in %q", text)
	}
	if !strings.Contains(text, "NY, USA") {
		t.Errorf("expected address in %q", text)
	}
	if !strings.Contains(text, "40.714268") {
		t.Errorf("expected latitude in %q", text)
	}
}

func TestFormatLocationMessage_Minimal(t *testing.T) {
	loc := &kapso.LocationContent{
		Latitude:  0,
		Longitude: 0,
	}
	text := formatLocationMessage(loc)
	want := fmt.Sprintf("[location] (%.6f, %.6f)", 0.0, 0.0)
	if text != want {
		t.Fatalf("got %q, want %q", text, want)
	}
}

func TestInboundMessageJSON_Image(t *testing.T) {
	raw := `{
		"id": "wamid.abc",
		"type": "image",
		"from": "1234567890",
		"timestamp": "1740704195",
		"image": {
			"id": "media-img-1",
			"mime_type": "image/jpeg",
			"caption": "Look at this"
		}
	}`
	var msg kapso.InboundMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "image" {
		t.Fatalf("type = %q, want image", msg.Type)
	}
	if msg.Image == nil {
		t.Fatal("Image field is nil")
	}
	if msg.Image.Caption != "Look at this" {
		t.Errorf("caption = %q, want %q", msg.Image.Caption, "Look at this")
	}
}

func TestInboundMessageJSON_Document(t *testing.T) {
	raw := `{
		"id": "wamid.def",
		"type": "document",
		"from": "1234567890",
		"timestamp": "1740704195",
		"document": {
			"id": "media-doc-1",
			"mime_type": "application/pdf",
			"filename": "invoice.pdf"
		}
	}`
	var msg kapso.InboundMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Document == nil || msg.Document.Filename != "invoice.pdf" {
		t.Fatal("document not parsed correctly")
	}
}

// rewriteTransport rewrites all request URLs to point at the test server.
type rewriteTransport struct {
	base    string
	wrapped http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.base, "http://")
	return t.wrapped.RoundTrip(req)
}
