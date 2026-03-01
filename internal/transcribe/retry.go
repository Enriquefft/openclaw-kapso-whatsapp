package transcribe

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// isRetryable reports whether the error warrants a retry attempt.
// Only *httpError with status 429 or 5xx is retryable.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var he *httpError
	if !errors.As(err, &he) {
		return false
	}
	return he.StatusCode == 429 || he.StatusCode >= 500
}

// retryTranscriber wraps a Transcriber with exponential-backoff retry logic.
type retryTranscriber struct {
	inner     Transcriber
	attempts  int
	base      time.Duration
	factor    float64
	jitter    float64
	timeout   time.Duration
	sleepFunc func(time.Duration)
}

// newRetryTranscriber creates a retryTranscriber with sensible defaults.
// timeout of 0 means no per-transcription timeout is applied.
func newRetryTranscriber(inner Transcriber, timeout time.Duration) *retryTranscriber {
	return &retryTranscriber{
		inner:     inner,
		attempts:  3,
		base:      1 * time.Second,
		factor:    2.0,
		jitter:    0.25,
		timeout:   timeout,
		sleepFunc: time.Sleep,
	}
}

// Transcribe calls the inner Transcriber, retrying on transient errors up to
// r.attempts times with exponential backoff and jitter. A per-transcription
// timeout wraps the entire retry span when r.timeout > 0.
func (r *retryTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	// Check context before starting; handles pre-cancelled context case.
	if err := ctx.Err(); err != nil {
		return "", err
	}

	delay := r.base
	var lastErr error

	for i := 0; i < r.attempts; i++ {
		text, err := r.inner.Transcribe(ctx, audio, mimeType)
		if err == nil {
			return text, nil
		}
		lastErr = err

		// Do not retry if error is non-retryable or this is the last attempt.
		if !isRetryable(err) || i == r.attempts-1 {
			break
		}

		// Respect context cancellation before sleeping.
		if ctx.Err() != nil {
			break
		}

		// Apply jitter: delay * (1 + random in [0, jitter)).
		jitterAmount := time.Duration(float64(delay) * r.jitter * rand.Float64()) //nolint:gosec
		r.sleepFunc(delay + jitterAmount)
		delay = time.Duration(float64(delay) * r.factor)
	}

	// If context was cancelled, return the context error for clarity.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}

	if !isRetryable(lastErr) {
		return "", lastErr
	}

	return "", fmt.Errorf("transcribe failed after %d attempts: %w", r.attempts, lastErr)
}
