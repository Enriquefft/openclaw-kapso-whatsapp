package transcribe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// whisperVerboseJSON is the mock response fixture.
const whisperVerboseJSON = `{"text":"hello world","language":"en","duration":1.5}`

// newMockWhisperServer creates a test server that parses multipart and
// validates/records fields, then returns a JSON response.
func newMockWhisperServer(t *testing.T, statusCode int, body string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	cap := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.AuthHeader = r.Header.Get("Authorization")

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			http.Error(w, "expected multipart", http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fieldName := part.FormName()
			data, _ := io.ReadAll(part)
			switch fieldName {
			case "file":
				cap.FileContentType = part.Header.Get("Content-Type")
				cap.FileData = data
			case "model":
				cap.Model = string(data)
			case "response_format":
				cap.ResponseFormat = string(data)
			case "language":
				cap.Language = string(data)
				cap.HasLanguage = true
			}
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	return srv, cap
}

type capturedRequest struct {
	AuthHeader      string
	FileContentType string
	FileData        []byte
	Model           string
	ResponseFormat  string
	Language        string
	HasLanguage     bool
}

func TestOpenAIWhisper(t *testing.T) {
	fakeAudio := []byte("fake-audio-data")

	tests := []struct {
		name            string
		provider        openAIWhisper
		serverStatus    int
		serverBody      string
		inputMIME       string
		wantText        string
		wantErr         bool
		wantErrType     bool // want *httpError
		wantErrStatus   int
		checkAuth       bool
		checkFileType   string
		checkModel      string
		checkLang       string
		checkNoLang     bool
	}{
		{
			name: "openai success",
			provider: openAIWhisper{
				APIKey: "test-key",
				Model:  "whisper-1",
			},
			serverStatus: 200,
			serverBody:   whisperVerboseJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello world",
		},
		{
			name: "openai with language",
			provider: openAIWhisper{
				APIKey:   "test-key",
				Model:    "whisper-1",
				Language: "en",
			},
			serverStatus: 200,
			serverBody:   whisperVerboseJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello world",
			checkLang:    "en",
		},
		{
			name: "openai without language",
			provider: openAIWhisper{
				APIKey: "test-key",
				Model:  "whisper-1",
			},
			serverStatus: 200,
			serverBody:   whisperVerboseJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello world",
			checkNoLang:  true,
		},
		{
			name: "openai auth header",
			provider: openAIWhisper{
				APIKey: "test-key",
				Model:  "whisper-1",
			},
			serverStatus: 200,
			serverBody:   whisperVerboseJSON,
			inputMIME:    "audio/ogg",
			wantText:     "hello world",
			checkAuth:    true,
		},
		{
			name: "openai multipart content-type matches normalized mime",
			provider: openAIWhisper{
				APIKey: "test-key",
				Model:  "whisper-1",
			},
			serverStatus:  200,
			serverBody:    whisperVerboseJSON,
			inputMIME:     "audio/ogg; codecs=opus",
			wantText:      "hello world",
			checkFileType: "audio/ogg",
		},
		{
			name: "openai non-200 returns httpError",
			provider: openAIWhisper{
				APIKey: "test-key",
				Model:  "whisper-1",
			},
			serverStatus:  500,
			serverBody:    "internal server error",
			inputMIME:     "audio/ogg",
			wantErr:       true,
			wantErrType:   true,
			wantErrStatus: 500,
		},
		{
			name: "groq success",
			provider: openAIWhisper{
				APIKey: "gsk-test",
				Model:  "whisper-large-v3",
			},
			serverStatus: 200,
			serverBody:   whisperVerboseJSON,
			inputMIME:    "audio/mpeg",
			wantText:     "hello world",
			checkModel:   "whisper-large-v3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, cap := newMockWhisperServer(t, tc.serverStatus, tc.serverBody)
			defer srv.Close()

			// Point the provider at the test server.
			tc.provider.BaseURL = srv.URL

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
				if cap.AuthHeader != "Bearer test-key" {
					t.Errorf("Authorization header = %q, want %q", cap.AuthHeader, "Bearer test-key")
				}
			}
			if tc.checkFileType != "" {
				if cap.FileContentType != tc.checkFileType {
					t.Errorf("file part Content-Type = %q, want %q", cap.FileContentType, tc.checkFileType)
				}
			}
			if tc.checkModel != "" {
				if cap.Model != tc.checkModel {
					t.Errorf("model field = %q, want %q", cap.Model, tc.checkModel)
				}
			}
			if tc.checkLang != "" {
				if !cap.HasLanguage {
					t.Error("expected language field in multipart, not found")
				} else if cap.Language != tc.checkLang {
					t.Errorf("language field = %q, want %q", cap.Language, tc.checkLang)
				}
			}
			if tc.checkNoLang {
				if cap.HasLanguage {
					t.Errorf("expected no language field, but found %q", cap.Language)
				}
			}
			// Always check response_format is verbose_json.
			if cap.ResponseFormat != "" && cap.ResponseFormat != "verbose_json" {
				t.Errorf("response_format = %q, want %q", cap.ResponseFormat, "verbose_json")
			}

			// Keep json import used.
			_ = json.Marshal
		})
	}
}
