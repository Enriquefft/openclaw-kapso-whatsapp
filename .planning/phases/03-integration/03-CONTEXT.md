# Phase 3: Integration - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Audio messages flow end-to-end from WhatsApp through transcription into the relay pipeline. Implement the local whisper.cpp provider, wire transcription into `delivery.ExtractText`, and connect the transcriber from `main.go` startup. No new providers — cloud providers (openai, groq, deepgram) are already wired from Phase 2.

</domain>

<decisions>
## Implementation Decisions

### Failure behavior
- When transcription fails for any reason, pipeline receives `[audio] (mime)` with a WARN log — message is never lost (matches roadmap criterion #2)
- No WhatsApp notification to sender on failure — silent fallback, consistent with existing media handling patterns
- Use existing provider-level timeout (30s config default) — no separate pipeline timeout needed

### Output formatting
- Successfully transcribed audio appears as `[voice] <transcript>` — matches roadmap criterion #1 exactly
- Distinct from `[audio]` fallback tag so agent knows transcription succeeded
- No language tag in output — language is a config detail, not useful to the agent

### Local whisper.cpp provider
- `ModelPath` is required config — user must set `model_path` or `KAPSO_TRANSCRIBE_MODEL_PATH`, clear error if missing. No auto-detection magic, consistent with project's explicit config philosophy
- Temp files for OGG→WAV conversion use `os.TempDir()` — standard Go approach, cleaned up on completion and context cancellation (roadmap criterion #3)
- Validate both `whisper-cli` and `ffmpeg` at startup in `New()` — fail fast, consistent with existing `exec.LookPath` pattern already in the factory

### Audio size limits
- Oversized audio (>25MB default) skips transcription and falls back to `[audio] (mime)` with WARN log — treated as transcription failure
- Size check happens after download — simpler, always works, Kapso media metadata may not reliably provide Content-Length

### Claude's Discretion
- Exact ffmpeg conversion flags for OGG→WAV
- Whisper-cli invocation flags and output parsing
- Error message wording in logs
- Whether to use a streaming or buffered approach for audio download

</decisions>

<specifics>
## Specific Ideas

- `ExtractText` signature must accept a nil Transcriber and preserve current behavior unchanged for all non-audio messages (roadmap criterion #4)
- `main.go` builds Transcriber from config at startup — nil if provider is unconfigured (roadmap criterion #5). The `_ = transcriber` discard in main.go gets replaced with actual wiring.
- Local provider converts OGG to WAV via ffmpeg, runs whisper-cli, cleans up temp files (roadmap criterion #3)

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `transcribe.Transcriber` interface: `Transcribe(ctx, audio, mimeType) (string, error)` — already defined
- `transcribe.New()` factory: local case exists as stub (`"not yet implemented (Phase 3)"`) — replace with implementation
- `transcribe.newRetryTranscriber()`: wraps cloud providers — local provider should NOT use retry wrapper (local failures aren't transient)
- `config.TranscribeConfig`: `BinaryPath`, `ModelPath`, `Language`, `MaxAudioSize`, `Timeout` all ready

### Established Patterns
- Factory pattern in `transcribe.New()` with `exec.LookPath` validation at startup
- `delivery.ExtractText` is a pure function taking `(msg, client)` — adding Transcriber changes its signature
- Media download via `client.GetMediaURL(mediaID)` returns URL, then HTTP GET to download bytes
- Table-driven tests with dependency injection throughout

### Integration Points
- `main.go:42` — `_ = transcriber` placeholder, needs to pass transcriber to delivery layer
- `delivery/extract.go:39-43` — audio case currently formats as `[audio]`, needs transcription branch
- `kapso/client.go` — `GetMediaURL` returns media URL for downloading audio bytes

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-integration*
*Context gathered: 2026-03-01*
