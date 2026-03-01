package kapso

import (
	"fmt"
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

func TestDownloadMedia(t *testing.T) {
	t.Run("under limit returns full body", func(t *testing.T) {
		body := []byte("hello audio data")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		got, err := client.DownloadMedia("http://example.com/media/audio.ogg", int64(len(body)+100))
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
			w.Write(body)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		got, err := client.DownloadMedia("http://example.com/media/audio.ogg", int64(len(body)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(got) != string(body) {
			t.Errorf("got %q, want %q", got, body)
		}
	})

	t.Run("exceeds limit by 1 byte returns size limit error", func(t *testing.T) {
		// Write maxBytes+1 bytes of data in the response.
		maxBytes := int64(10)
		body := make([]byte, maxBytes+1)
		for i := range body {
			body[i] = 'x'
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		_, err := client.DownloadMedia("http://example.com/media/audio.ogg", maxBytes)
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
			w.Write([]byte("audio"))
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        wantKey,
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		_, err := client.DownloadMedia("http://example.com/media/audio.ogg", 1024)
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
			fmt.Fprint(w, "forbidden")
		}))
		defer srv.Close()

		client := &Client{
			APIKey:        "test-key",
			PhoneNumberID: "12345",
			HTTPClient:    &http.Client{Transport: &rewriteTransport{base: srv.URL, wrapped: http.DefaultTransport}},
		}

		_, err := client.DownloadMedia("http://example.com/media/audio.ogg", 1024)
		if err == nil {
			t.Fatal("expected error for non-200 status, got nil")
		}
		if !strings.Contains(err.Error(), "403") {
			t.Errorf("error %q does not contain status code 403", err.Error())
		}
	})
}
