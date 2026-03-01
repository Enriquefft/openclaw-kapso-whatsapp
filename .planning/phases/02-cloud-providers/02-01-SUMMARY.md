---
phase: 02-cloud-providers
plan: "01"
subsystem: transcription
tags: [openai, groq, whisper, multipart, mime, http, tdd]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: Transcriber interface, TranscribeConfig, config package
provides:
  - openAIWhisper struct implementing Transcriber for OpenAI and Groq via configurable BaseURL
  - NormalizeMIME helper mapping all OGG/Opus variants to canonical audio/ogg
  - mimeToFilename helper for correct file extension in multipart uploads
  - httpError type for non-200 provider responses
  - Factory New() returning real working providers for openai and groq cases
affects: [02-02-retry, 02-03-deepgram, 03-pipeline]

# Tech tracking
tech-stack:
  added: [mime/multipart, net/textproto, httptest (test), net/http/httptest (test)]
  patterns:
    - Shared struct with configurable BaseURL for provider variants (no code duplication between OpenAI and Groq)
    - CreatePart with explicit textproto.MIMEHeader for correct Content-Type on multipart file parts
    - NormalizeMIME called before provider-specific logic — single normalization point
    - verbose_json response format with result.Text extraction
    - httpError sentinel type enabling errors.As assertions in tests and callers

key-files:
  created:
    - internal/transcribe/mime.go
    - internal/transcribe/mime_test.go
    - internal/transcribe/openai.go
    - internal/transcribe/openai_test.go
  modified:
    - internal/transcribe/transcribe.go
    - internal/transcribe/transcribe_test.go

key-decisions:
  - "openAIWhisper uses CreatePart+textproto.MIMEHeader instead of CreateFormFile — CreateFormFile hardcodes application/octet-stream which causes Whisper API to reject audio files"
  - "w.Close() called explicitly before http.NewRequestWithContext — defer would leave buffer incomplete at request construction time"
  - "verbose_json response format chosen over json — provides text, language, duration metadata even though only text is used now"
  - "audio/opus normalized to audio/ogg — Kapso sends audio/ogg;codecs=opus, Whisper API needs the base type"

patterns-established:
  - "MIME normalization: always call NormalizeMIME before any provider-specific MIME usage"
  - "Provider variants: single struct + configurable BaseURL, not separate types per provider"
  - "Multipart file part: use CreatePart with explicit Content-Type, never CreateFormFile"
  - "httpError type: structured error with StatusCode for programmatic error handling"

requirements-completed: [PROV-01, PROV-02, PROV-04, INFR-03, TEST-01]

# Metrics
duration: 2min
completed: 2026-03-01
---

# Phase 02 Plan 01: OpenAI/Groq Transcription Provider Summary

**OpenAI-compatible Whisper provider (shared struct, configurable BaseURL) with MIME normalization layer wired into factory for both openai and groq cases**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-01T15:10:49Z
- **Completed:** 2026-03-01T15:13:17Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- MIME normalization: `NormalizeMIME` strips params and maps `audio/opus`->`audio/ogg`, all other types pass through stripped
- `mimeToFilename` maps normalized MIME to correct filename extension (ogg, mp3, mp4, wav, webm, flac, bin fallback)
- `openAIWhisper` struct with `Transcribe` using `CreatePart`+`textproto.MIMEHeader` for correct Content-Type on file parts (INFR-03)
- `httpError` type with `StatusCode` for structured error handling on non-200 responses
- Factory `New()` now returns real `openAIWhisper` instances for openai (whisper-1 default) and groq (whisper-large-v3 default)
- Table-driven tests against `httptest` mock servers verify auth header, file Content-Type, language field presence/absence, model field, and error type

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for MIME and OpenAI/Groq** - `2e69f41` (test)
2. **Task 1 GREEN: MIME helpers and OpenAI/Groq provider** - `7b92aab` (feat)
3. **Task 2: Wire providers into factory** - `1a93bc5` (feat)

_Note: TDD task has RED commit (test) and GREEN commit (feat)_

## Files Created/Modified
- `internal/transcribe/mime.go` - NormalizeMIME and mimeToFilename helpers
- `internal/transcribe/mime_test.go` - Table-driven tests for MIME normalization and filename mapping
- `internal/transcribe/openai.go` - openAIWhisper struct, httpError type, Transcribe method
- `internal/transcribe/openai_test.go` - Table-driven tests with httptest mock server for all provider scenarios
- `internal/transcribe/transcribe.go` - Updated factory: openai/groq return real providers, deepgram keeps stub
- `internal/transcribe/transcribe_test.go` - Updated: openai/groq with key now expect non-nil transcriber

## Decisions Made
- `CreatePart` with explicit `textproto.MIMEHeader` chosen over `CreateFormFile` — the latter hardcodes `application/octet-stream` which causes Whisper API to fail; explicit header sets the actual audio MIME type
- `w.Close()` called before `http.NewRequestWithContext` (not deferred) — buffer must be complete before the request is constructed and the body reader is set
- `verbose_json` response format: richer than `json`, gives language and duration metadata at no extra cost
- `audio/opus` maps to `audio/ogg` in `NormalizeMIME` — Kapso Cloud API sends `audio/ogg; codecs=opus`, Whisper API needs the base MIME type without params

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. API keys are runtime config, not build-time.

## Next Phase Readiness
- OpenAI and Groq providers ready for integration testing with real API keys
- Factory returns working instances — Phase 3 pipeline can call `transcribe.New()` and get a functional provider
- Retry/resilience wrapper (Plan 02-02) can wrap the returned `openAIWhisper` instance
- Deepgram stub still in place — Plan 02-02 or 02-03 will replace it

---
*Phase: 02-cloud-providers*
*Completed: 2026-03-01*
