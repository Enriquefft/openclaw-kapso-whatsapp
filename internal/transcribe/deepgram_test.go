package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// deepgramSuccessJSON is a minimal valid Deepgram v1/listen response.
const deepgramSuccessJSON = `{
  "results": {
    "channels": [
      {
        "alternatives": [
          {
            "transcript": "hello deepgram"
          }
        ]
      }
    ]
  }
}`

// deepgramEmptyChannelsJSON simulates a response with no channels.
const deepgramEmptyChannelsJSON = `{"results":{"channels":[]}}`

// deepgramEmptyAlternativesJSON simulates a response with channels but no alternatives.
const deepgramEmptyAlternativesJSON = `{"results":{"channels":[{"alternatives":[]}]}}`

type capturedDeepgramRequest struct {
	AuthHeader  string
	ContentType string
	Body        []byte
	QueryParams url.Values
}

func newMockDeepgramServer(t *testing.T, statusCode int, body string) (*httptest.Server, *capturedDeepgramRequest) {
	t.Helper()
	cap := &capturedDeepgramRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.AuthHeader = r.Header.Get("Authorization")
		cap.ContentType = r.Header.Get("Content-Type")
		cap.QueryParams = r.URL.Query()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cap.Body = data
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	return srv, cap
}

func TestDeepgram(t *testing.T) {
	fakeAudio := []byte("fake-audio-bytes")

	tests := []struct {
		name             string
		provider         deepgramProvider
		serverStatus     int
		serverBody       string
		inputMIME        string
		wantText         string
		wantErr          bool
		wantErrContains  string
		wantErrType      bool // want *httpError
		wantErrStatus    int
		checkAuth        bool
		checkContentType string
		checkModel       string
		checkSmartFmt    bool
		checkLang        string
		checkNoLang      bool
		checkRawBody     bool // body is raw bytes (not multipart)
	}{
		{
			name: "success",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus: 200,
			serverBody:   deepgramSuccessJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello deepgram",
		},
		{
			name: "auth header uses Token not Bearer",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus: 200,
			serverBody:   deepgramSuccessJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello deepgram",
			checkAuth:    true,
		},
		{
			name: "content-type header matches normalized MIME",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus:     200,
			serverBody:       deepgramSuccessJSON,
			inputMIME:        "audio/ogg; codecs=opus",
			wantText:         "hello deepgram",
			checkContentType: "audio/ogg",
		},
		{
			name: "query params include model and smart_format",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus:  200,
			serverBody:    deepgramSuccessJSON,
			inputMIME:     "audio/ogg",
			wantText:      "hello deepgram",
			checkModel:    "nova-3",
			checkSmartFmt: true,
		},
		{
			name: "query params include language when set",
			provider: deepgramProvider{
				APIKey:   "test-key",
				Model:    "nova-3",
				Language: "es",
			},
			serverStatus: 200,
			serverBody:   deepgramSuccessJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello deepgram",
			checkLang:    "es",
		},
		{
			name: "no language param when Language is empty",
			provider: deepgramProvider{
				APIKey:   "test-key",
				Model:    "nova-3",
				Language: "",
			},
			serverStatus: 200,
			serverBody:   deepgramSuccessJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello deepgram",
			checkNoLang:  true,
		},
		{
			name: "raw binary body (not multipart)",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus: 200,
			serverBody:   deepgramSuccessJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello deepgram",
			checkRawBody: true,
		},
		{
			name: "empty channels returns error",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus:    200,
			serverBody:      deepgramEmptyChannelsJSON,
			inputMIME:       "audio/ogg",
			wantErr:         true,
			wantErrContains: "missing channels",
		},
		{
			name: "empty alternatives returns error",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus:    200,
			serverBody:      deepgramEmptyAlternativesJSON,
			inputMIME:       "audio/ogg",
			wantErr:         true,
			wantErrContains: "missing channels",
		},
		{
			name: "non-200 returns httpError",
			provider: deepgramProvider{
				APIKey: "test-key",
				Model:  "nova-3",
			},
			serverStatus:  503,
			serverBody:    "service unavailable",
			inputMIME:     "audio/ogg",
			wantErr:       true,
			wantErrType:   true,
			wantErrStatus: 503,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, cap := newMockDeepgramServer(t, tc.serverStatus, tc.serverBody)
			defer srv.Close()

			// Override the provider URL via a test-injected HTTP client pointing to test server.
			tc.provider.BaseURL = srv.URL + "/v1/listen"

			got, err := tc.provider.Transcribe(context.Background(), fakeAudio, tc.inputMIME)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrType {
					var he *httpError
					if !errors.As(err, &he) {
						t.Fatalf("expected *httpError, got %T: %v", err, err)
					}
					if he.StatusCode != tc.wantErrStatus {
						t.Errorf("httpError.StatusCode = %d, want %d", he.StatusCode, tc.wantErrStatus)
					}
				}
				if tc.wantErrContains != "" && err != nil {
					if !containsStr(err.Error(), tc.wantErrContains) {
						t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrContains)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantText {
				t.Errorf("Transcribe() = %q, want %q", got, tc.wantText)
			}

			// Additional assertions.
			if tc.checkAuth {
				want := "Token test-key"
				if cap.AuthHeader != want {
					t.Errorf("Authorization header = %q, want %q", cap.AuthHeader, want)
				}
			}
			if tc.checkContentType != "" {
				if cap.ContentType != tc.checkContentType {
					t.Errorf("Content-Type header = %q, want %q", cap.ContentType, tc.checkContentType)
				}
			}
			if tc.checkModel != "" {
				if cap.QueryParams.Get("model") != tc.checkModel {
					t.Errorf("query param model = %q, want %q", cap.QueryParams.Get("model"), tc.checkModel)
				}
			}
			if tc.checkSmartFmt {
				if cap.QueryParams.Get("smart_format") != "true" {
					t.Errorf("query param smart_format = %q, want %q", cap.QueryParams.Get("smart_format"), "true")
				}
			}
			if tc.checkLang != "" {
				if cap.QueryParams.Get("language") != tc.checkLang {
					t.Errorf("query param language = %q, want %q", cap.QueryParams.Get("language"), tc.checkLang)
				}
			}
			if tc.checkNoLang {
				if cap.QueryParams.Has("language") {
					t.Errorf("expected no language query param, but found %q", cap.QueryParams.Get("language"))
				}
			}
			if tc.checkRawBody {
				if !bytes.Equal(cap.Body, fakeAudio) {
					t.Errorf("request body = %q, want raw audio bytes %q", cap.Body, fakeAudio)
				}
			}

			// Keep json import used.
			_ = json.Marshal
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
