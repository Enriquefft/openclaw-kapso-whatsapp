---
phase: 02-cloud-providers
plan: 02
subsystem: transcribe
tags: [deepgram, retry, exponential-backoff, tdd]
dependency_graph:
  requires: ["02-01"]
  provides: ["02-03"]
  affects: ["internal/transcribe"]
tech_stack:
  added: []
  patterns:
    - "Binary POST body (not multipart) for Deepgram audio upload"
    - "Token auth header scheme (Deepgram-specific, not Bearer)"
    - "Exponential backoff with jitter via injectable sleepFunc for testability"
    - "Decorator pattern: retryTranscriber wraps any Transcriber"
    - "Internal package test file for unexported type assertions"
key_files:
  created:
    - internal/transcribe/deepgram.go
    - internal/transcribe/deepgram_test.go
    - internal/transcribe/retry.go
    - internal/transcribe/retry_test.go
    - internal/transcribe/factory_internal_test.go
  modified:
    - internal/transcribe/transcribe.go
    - internal/transcribe/transcribe_test.go
decisions:
  - "deepgramProvider.BaseURL field overridable in tests — avoids global URL constant, keeps struct self-contained"
  - "retryTranscriber.sleepFunc injectable for zero-delay tests — same pattern as mockable now() from Phase 1"
  - "factory_internal_test.go as separate internal-package file — allows type assertion on unexported *retryTranscriber"
  - "isRetryable returns false for nil and non-httpError errors — non-HTTP errors (network failures) not automatically retried"
metrics:
  duration: "3min"
  completed_date: "2026-03-01"
  tasks_completed: 2
  files_created: 5
  files_modified: 2
---

# Phase 2 Plan 2: Deepgram Provider and Retry Infrastructure Summary

Deepgram transcription provider (binary POST, Token auth, query params) and retryTranscriber wrapper (exponential backoff with jitter on 429/5xx) implemented with TDD. All three cloud providers now wrapped with retry in the factory.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Deepgram provider and retry wrapper | dfef7f3 | deepgram.go, retry.go (+ failing tests 5dbb370) |
| 2 | Wire Deepgram and retry wrapper into factory | 02e6906 | transcribe.go, factory_internal_test.go |

## What Was Built

**deepgramProvider** (`internal/transcribe/deepgram.go`):
- Sends raw binary audio as POST body (not multipart) to `https://api.deepgram.com/v1/listen`
- Authorization header: `Token <APIKey>` (not `Bearer`)
- Query params: `model`, `smart_format=true`, optional `language`
- Parses nested response: `results.channels[0].alternatives[0].transcript`
- Errors on empty channels or alternatives arrays
- Returns `*httpError` on non-200 responses
- BaseURL field overridable in tests

**retryTranscriber** (`internal/transcribe/retry.go`):
- Wraps any `Transcriber` with up to 3 retry attempts
- Retries on `*httpError` with StatusCode 429 or >= 500 only
- Non-retryable errors (4xx except 429) fail immediately after 1 attempt
- Exponential backoff: 1s base, 2x factor, 25% jitter
- Injectable `sleepFunc` for zero-delay testing
- Per-transcription timeout wraps entire retry span (not per-attempt)
- Checks `ctx.Err()` before sleeping; returns context error on cancellation
- Exhaustion error: `"transcribe failed after 3 attempts: <last error>"`

**Factory update** (`internal/transcribe/transcribe.go`):
- `var p Transcriber` declared before switch; all cloud cases assign to `p`
- After switch: `return newRetryTranscriber(p, timeout), nil`
- Deepgram case: default model `"nova-3"`; model, language, APIKey from config
- local provider returns directly without retry wrapper

## Test Coverage

- `TestDeepgram` (10 cases): success, Token auth header, Content-Type, query params, language present/absent, raw body, empty channels, empty alternatives, non-200 httpError
- `TestRetryTranscriber` (7 cases): success first try, retry on 429, retry on 5xx, no retry on 400, no retry on 401, exhaustion message, context cancellation
- `TestIsRetryable` (10 cases): nil, non-http, 429, 500, 502, 503, 400, 401, 403, 404
- `TestNewWrapsCloudProvidersWithRetry` (3 cases): openai, groq, deepgram all return `*retryTranscriber`
- All existing tests (TestNew, TestOpenAIWhisper, TestNormalizeMIME, TestMimeToFilename) still pass

## Verification

```
go test ./internal/transcribe/ -v — PASS (42 tests)
go vet ./internal/transcribe/ — clean
go build ./cmd/kapso-whatsapp-poller/ — success
grep "Token" deepgram.go — Authorization: "Token "+p.APIKey confirmed
grep "newRetryTranscriber" transcribe.go — factory wrapping confirmed
```

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED
