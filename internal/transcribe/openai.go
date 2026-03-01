package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// httpError is returned when the provider responds with a non-200 status code.
type httpError struct {
	StatusCode int
	Body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("provider returned %d: %s", e.StatusCode, e.Body)
}

// noSpeechError is returned when the no_speech_prob metric exceeds the
// configured threshold, indicating likely silence or noise rather than speech.
type noSpeechError struct {
	Prob      float64
	Threshold float64
}

func (e *noSpeechError) Error() string {
	return fmt.Sprintf("no_speech_prob %.4f exceeds threshold %.2f", e.Prob, e.Threshold)
}

// openAIWhisper implements Transcriber for both OpenAI and Groq via the
// OpenAI-compatible Whisper API. Use different BaseURLs to target each service.
type openAIWhisper struct {
	BaseURL           string
	APIKey            string
	Model             string
	Language          string
	NoSpeechThreshold float64
	Debug             bool
	HTTPClient        *http.Client
}

// whisperVerboseResponse holds the full verbose_json response from the Whisper API.
type whisperVerboseResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
	Segments []struct {
		AvgLogprob   float64 `json:"avg_logprob"`
		NoSpeechProb float64 `json:"no_speech_prob"`
	} `json:"segments"`
}

// client returns the configured HTTP client or the default one.
func (p *openAIWhisper) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return http.DefaultClient
}

// providerName derives the provider name from BaseURL for logging.
func (p *openAIWhisper) providerName() string {
	if strings.Contains(p.BaseURL, "groq") {
		return "groq"
	}
	return "openai"
}

// Transcribe sends audio bytes to the Whisper API and returns the transcript.
func (p *openAIWhisper) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	norm := NormalizeMIME(mimeType)
	filename := mimeToFilename(norm)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Create the file part with explicit Content-Type using CreatePart (not
	// CreateFormFile which always uses application/octet-stream).
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", norm)
	filePart, err := w.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("create file part: %w", err)
	}
	if _, err = filePart.Write(audio); err != nil {
		return "", fmt.Errorf("write audio data: %w", err)
	}

	if err = w.WriteField("model", p.Model); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	if err = w.WriteField("response_format", "verbose_json"); err != nil {
		return "", fmt.Errorf("write response_format field: %w", err)
	}
	if p.Language != "" {
		if err = w.WriteField("language", p.Language); err != nil {
			return "", fmt.Errorf("write language field: %w", err)
		}
	}

	// Close before building request — buffer must be complete.
	if err = w.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := p.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &httpError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result whisperVerboseResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	// Compute metrics across segments.
	var maxNoSpeech, sumLogprob float64
	for _, seg := range result.Segments {
		if seg.NoSpeechProb > maxNoSpeech {
			maxNoSpeech = seg.NoSpeechProb
		}
		sumLogprob += seg.AvgLogprob
	}
	var avgLogprob float64
	if len(result.Segments) > 0 {
		avgLogprob = sumLogprob / float64(len(result.Segments))
	}

	// Debug logging: emit quality metrics when enabled.
	if p.Debug {
		log.Printf("[transcribe:debug] provider=%s model=%s language=%s avg_logprob=%.4f no_speech_prob=%.4f duration_ms=%.0f",
			p.providerName(), p.Model, result.Language, avgLogprob, maxNoSpeech, result.Duration*1000)
	}

	// No-speech guard: reject likely-silent audio to prevent hallucinated transcripts.
	// Only applied when segments are present (empty segments = short clip, skip guard).
	if len(result.Segments) > 0 && p.NoSpeechThreshold > 0 && maxNoSpeech >= p.NoSpeechThreshold {
		log.Printf("[transcribe:warn] no_speech_prob %.4f >= threshold %.2f — rejecting transcript to prevent hallucination",
			maxNoSpeech, p.NoSpeechThreshold)
		return "", &noSpeechError{Prob: maxNoSpeech, Threshold: p.NoSpeechThreshold}
	}

	return result.Text, nil
}
