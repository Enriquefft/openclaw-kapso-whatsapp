package delivery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/kapso"
)

func TestExtractText_Text(t *testing.T) {
	msg := kapso.Message{
		ID:   "m1",
		Type: "text",
		From: "+1234567890",
		Text: &kapso.TextContent{Body: "hello world"},
	}
	text, ok := ExtractText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true for text message")
	}
	if text != "hello world" {
		t.Fatalf("got %q, want %q", text, "hello world")
	}
}

func TestExtractText_TextNilBody(t *testing.T) {
	msg := kapso.Message{
		ID:   "m2",
		Type: "text",
		From: "+1234567890",
	}
	_, ok := ExtractText(msg, nil)
	if ok {
		t.Fatal("expected ok=false for text message with nil Text")
	}
}

func TestExtractText_Image(t *testing.T) {
	msg := kapso.Message{
		ID:   "m3",
		Type: "image",
		From: "+1234567890",
		Image: &kapso.ImageContent{
			ID:       "media-123",
			MimeType: "image/jpeg",
			Caption:  "sunset photo",
		},
	}
	text, ok := ExtractText(msg, nil)
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

func TestExtractText_ImageWithMediaURL(t *testing.T) {
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
		HTTPClient: &http.Client{
			Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport},
		},
	}

	msg := kapso.Message{
		ID:   "m3b",
		Type: "image",
		From: "+1234567890",
		Image: &kapso.ImageContent{
			ID:       "media-123",
			MimeType: "image/jpeg",
			Caption:  "sunset",
		},
	}
	text, ok := ExtractText(msg, client)
	if !ok {
		t.Fatal("expected ok=true for image message")
	}
	if !strings.Contains(text, "https://example.com/media/photo.jpg") {
		t.Errorf("expected media URL in %q", text)
	}
}

func TestExtractText_Document(t *testing.T) {
	msg := kapso.Message{
		ID:   "m4",
		Type: "document",
		From: "+1234567890",
		Document: &kapso.DocumentContent{
			ID:       "media-456",
			MimeType: "application/pdf",
			Filename: "report.pdf",
		},
	}
	text, ok := ExtractText(msg, nil)
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

func TestExtractText_DocumentCaptionFallback(t *testing.T) {
	msg := kapso.Message{
		ID:   "m4b",
		Type: "document",
		From: "+1234567890",
		Document: &kapso.DocumentContent{
			ID:       "media-456",
			MimeType: "application/pdf",
			Caption:  "my report",
		},
	}
	text, ok := ExtractText(msg, nil)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.Contains(text, "my report") {
		t.Errorf("expected caption fallback in %q", text)
	}
}

func TestExtractText_Audio(t *testing.T) {
	msg := kapso.Message{
		ID:   "m5",
		Type: "audio",
		From: "+1234567890",
		Audio: &kapso.AudioContent{
			ID:       "media-789",
			MimeType: "audio/ogg",
		},
	}
	text, ok := ExtractText(msg, nil)
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

func TestExtractText_Video(t *testing.T) {
	msg := kapso.Message{
		ID:   "m6",
		Type: "video",
		From: "+1234567890",
		Video: &kapso.VideoContent{
			ID:       "media-v1",
			MimeType: "video/mp4",
			Caption:  "funny clip",
		},
	}
	text, ok := ExtractText(msg, nil)
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

func TestExtractText_Location(t *testing.T) {
	msg := kapso.Message{
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
	text, ok := ExtractText(msg, nil)
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

func TestExtractText_UnsupportedType(t *testing.T) {
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

	msg := kapso.Message{
		ID:   "m8",
		Type: "sticker",
		From: "+1234567890",
		Sticker: &kapso.StickerContent{
			ID:       "stk-1",
			MimeType: "image/webp",
		},
	}
	_, ok := ExtractText(msg, client)
	if ok {
		t.Fatal("expected ok=false for unsupported sticker type")
	}

	got := <-ch
	if got.to != "+1234567890" {
		t.Errorf("notification sent to %q, want %q", got.to, "+1234567890")
	}
	if !strings.Contains(got.body, "sticker") {
		t.Errorf("notification body %q should mention sticker", got.body)
	}
}

func TestExtractText_NilMediaContent(t *testing.T) {
	for _, typ := range []string{"image", "document", "audio", "video", "location"} {
		msg := kapso.Message{
			ID:   "nil-" + typ,
			Type: typ,
			From: "+1234567890",
		}
		_, ok := ExtractText(msg, nil)
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
