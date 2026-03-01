# Phase 2: Cloud Providers - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement Groq, OpenAI, and Deepgram transcription providers using stdlib HTTP. Replace the "not yet implemented" stubs in `transcribe.New()` with working cloud provider implementations. Includes retry infrastructure and MIME normalization.

</domain>

<decisions>
## Implementation Decisions

### Provider API behavior
- OpenAI and Groq share one struct with configurable `BaseURL` — no duplicated HTTP logic
- Auto-detect model defaults per provider: openai defaults to `whisper-1`, groq defaults to `whisper-large-v3`. User can override via config
- MIME-derived filename in multipart form: `audio/ogg` → `audio.ogg`, `audio/mpeg` → `audio.mp3`
- Omit language parameter entirely when config field is empty (let API auto-detect)
- Request `verbose_json` response format — parse transcript from response JSON. Enables future `no_speech_prob` quality guard (Phase 4) and debug logging without changing provider code later

### Error & retry strategy
- Retry as specified in requirements: 3 attempts, 1s base, 2x factor, random jitter up to 25%. Total max ~7s wait
- Retry logic lives as an interface wrapper (`retryTranscriber` wrapping any `Transcriber`), not inside each provider. Clean separation, independently testable
- Error on exhaustion: last error with attempt count — "transcribe failed after 3 attempts: 503 Service Unavailable"
- Per-transcription timeout (`context.WithTimeout` using `cfg.Timeout`) enforced inside the retry wrapper in this phase

### MIME handling
- Strip codec params, map known variants: `audio/ogg; codecs=opus` → `audio/ogg`, `audio/opus` → `audio/ogg`
- Unsupported MIME types: try anyway, pass normalized MIME to provider and let it decide. Log a warning. Only fail if provider rejects it
- MIME normalization function lives in `internal/transcribe/mime.go` — co-located with providers
- Multipart Content-Type header uses the normalized MIME type

### Deepgram differences
- Deepgram gets its own separate struct implementing `Transcriber` — different enough (binary body, query params, different response shape) that sharing would be forced
- Missing/empty channels in response: return error. Treat as failed transcription. Strict and predictable
- `smart_format=true` always on — better punctuation for chat messages with no downside
- One `api_key` config field for all providers — each provider formats its own auth header internally (OpenAI/Groq use `Bearer`, Deepgram uses `Token`)

### Claude's Discretion
- Exact multipart boundary construction details
- HTTP client configuration (timeouts, transport settings)
- Internal error type design
- Test helper organization

</decisions>

<specifics>
## Specific Ideas

- The `verbose_json` decision is forward-looking: Phase 4 needs `no_speech_prob` from the response, and INFR-04 needs `avg_logprob` and detected language for debug logging. Building verbose_json parsing now avoids touching provider code later.
- Deepgram's `smart_format` is always-on because our use case is chat messages where punctuation matters for readability.

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `transcribe.Transcriber` interface: `Transcribe(ctx, audio, mimeType) (string, error)` — providers implement this
- `transcribe.New()` factory: switch on provider string, already normalizes with `strings.ToLower` + `strings.TrimSpace`
- `config.TranscribeConfig`: has `Provider`, `APIKey`, `Model`, `Language`, `Timeout`, `MaxAudioSize` fields ready to use
- `kapso.Client` HTTP patterns: stdlib `net/http`, `X-API-Key` header, `httptest.NewServer` for tests

### Established Patterns
- Standard `log` package for logging (no frameworks)
- Table-driven tests with `httptest.NewServer` for HTTP mocking (see `internal/kapso/client_test.go`)
- Errors wrapped with `fmt.Errorf("context: %w", err)` for chain
- Context-based cancellation for all operations
- No globals — all state in structs

### Integration Points
- `transcribe.New()` factory switch statement: replace "not yet implemented" returns with actual provider construction
- Provider structs need `config.TranscribeConfig` fields (APIKey, Model, Language, Timeout)
- Retry wrapper wraps the provider before returning from `New()` — transparent to callers

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-cloud-providers*
*Context gathered: 2026-03-01*
