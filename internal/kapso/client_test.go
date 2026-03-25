package kapso

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestMarkRead(t *testing.T) {
	t.Run("sends correct payload without typing indicator", func(t *testing.T) {
		var gotBody []byte
		var gotContentType, gotAPIKey string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotContentType = r.Header.Get("Content-Type")
			gotAPIKey = r.Header.Get("X-API-Key")
			var err error
			gotBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		err := client.MarkRead("wamid.abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if gotContentType != "application/json" {
			t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
		}
		if gotAPIKey != "test-key" {
			t.Errorf("X-API-Key = %q, want %q", gotAPIKey, "test-key")
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(gotBody, &payload); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if payload["status"] != "read" {
			t.Errorf("status = %q, want %q", payload["status"], "read")
		}
		if payload["message_id"] != "wamid.abc123" {
			t.Errorf("message_id = %q, want %q", payload["message_id"], "wamid.abc123")
		}
		if payload["messaging_product"] != "whatsapp" {
			t.Errorf("messaging_product = %q, want %q", payload["messaging_product"], "whatsapp")
		}
		if _, ok := payload["typing_indicator"]; ok {
			t.Error("typing_indicator should be omitted for MarkRead")
		}
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		err := client.MarkRead("wamid.abc123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "403") {
			t.Errorf("error %q does not contain status code 403", err.Error())
		}
	})
}

func TestMarkReadWithTyping(t *testing.T) {
	t.Run("includes typing indicator in payload", func(t *testing.T) {
		var gotBody []byte

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			gotBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		err := client.MarkReadWithTyping("wamid.abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(gotBody, &payload); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if payload["status"] != "read" {
			t.Errorf("status = %q, want %q", payload["status"], "read")
		}
		if payload["message_id"] != "wamid.abc123" {
			t.Errorf("message_id = %q, want %q", payload["message_id"], "wamid.abc123")
		}

		ti, ok := payload["typing_indicator"].(map[string]interface{})
		if !ok {
			t.Fatal("typing_indicator missing or wrong type")
		}
		if ti["type"] != "text" {
			t.Errorf("typing_indicator.type = %q, want %q", ti["type"], "text")
		}
	})
}

func TestSanitizeMediaURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"allowed kapso host", "https://media.kapso.ai/files/audio.ogg", false},
		{"allowed whatsapp host", "https://mmg.whatsapp.net/v/audio.ogg", false},
		{"allowed fbcdn host", "https://scontent.fbcdn.net/v/audio.ogg", false},
		{"bare kapso domain", "https://kapso.ai/files/audio.ogg", false},
		{"HTTP scheme rejected", "http://media.kapso.ai/files/audio.ogg", true},
		{"disallowed host", "https://evil.example.com/audio.ogg", true},
		{"localhost rejected", "https://localhost/secret", true},
		{"IP address rejected", "https://127.0.0.1/secret", true},
		{"empty string", "", true},
		{"no scheme", "media.kapso.ai/audio.ogg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := sanitizeMediaURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeMediaURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if err == nil && u.String() != tt.url {
				t.Errorf("sanitizeMediaURL(%q).String() = %q, want round-trip", tt.url, u.String())
			}
		})
	}
}

func TestDownloadMedia(t *testing.T) {
	// All DownloadMedia tests use an allowed HTTPS URL; the rewriteTransport
	// redirects the actual request to the local test server.
	const allowedURL = "https://media.kapso.ai/audio.ogg"

	t.Run("under limit returns full body", func(t *testing.T) {
		body := []byte("hello audio data")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		got, err := client.DownloadMedia(allowedURL, int64(len(body)+100))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != string(body) {
			t.Errorf("got %q, want %q", got, body)
		}
	})

	t.Run("exactly at limit returns full body", func(t *testing.T) {
		body := []byte("exact limit data")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		got, err := client.DownloadMedia(allowedURL, int64(len(body)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != string(body) {
			t.Errorf("got %q, want %q", got, body)
		}
	})

	t.Run("exceeds limit by 1 byte returns size limit error", func(t *testing.T) {
		maxBytes := int64(10)
		body := make([]byte, maxBytes+1)
		for i := range body {
			body[i] = 'x'
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		_, err := client.DownloadMedia(allowedURL, maxBytes)
		if err == nil {
			t.Fatal("expected size limit error, got nil")
		}
		if !strings.Contains(err.Error(), "size limit") {
			t.Errorf("error %q does not mention size limit", err.Error())
		}
	})

	t.Run("sends X-API-Key header", func(t *testing.T) {
		wantKey := "my-api-key-12345"
		var gotKey string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotKey = r.Header.Get("X-API-Key")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("audio"))
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        wantKey,
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		_, err := client.DownloadMedia(allowedURL, 1024)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotKey != wantKey {
			t.Errorf("X-API-Key header = %q, want %q", gotKey, wantKey)
		}
	})

	t.Run("non-200 status returns error with status code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, "forbidden")
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		_, err := client.DownloadMedia(allowedURL, 1024)
		if err == nil {
			t.Fatal("expected error for non-200 status, got nil")
		}
		if !strings.Contains(err.Error(), "403") {
			t.Errorf("error %q does not contain status code 403", err.Error())
		}
	})

	t.Run("disallowed URL rejected before HTTP request", func(t *testing.T) {
		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    http.DefaultClient,
		}

		_, err := client.DownloadMedia("https://evil.example.com/steal", 1024)
		if err == nil {
			t.Fatal("expected error for disallowed URL, got nil")
		}
		if !strings.Contains(err.Error(), "not in the allowed list") {
			t.Errorf("error %q does not mention allowed list", err.Error())
		}
	})
}
