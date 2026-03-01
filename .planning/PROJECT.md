# Kapso WhatsApp — Voice Transcription

## What This Is

Voice message transcription for the WhatsApp-OpenClaw bridge. Incoming audio messages are automatically transcribed to text before forwarding to OpenClaw, preserving the channel-agnostic `delivery.Event.Text` contract. Two transcription architectures: HTTP API providers (Groq/OpenAI/Deepgram with configurable model) and local whisper.cpp binary for offline/privacy-first use.

## Core Value

Audio messages from WhatsApp users reach OpenClaw as usable text — transparently, reliably, with graceful fallback if transcription fails.

## Requirements

### Validated

- ✓ WhatsApp message polling and forwarding to OpenClaw — existing v0.1.0
- ✓ Kapso HTTP client with media URL retrieval — existing v0.1.0
- ✓ Delivery event pipeline (guard → gateway → relay) — existing v0.1.0
- ✓ 3-tier config system (defaults < file < env) — existing v0.1.0
- ✓ Security: allowlist, rate limiting, roles, session isolation — existing v0.1.0

### Active

- [ ] Transcriber interface with single method: `Transcribe(ctx, audio, mimeType) (string, error)`
- [ ] HTTP API provider: OpenAI Whisper (multipart form, configurable model)
- [ ] HTTP API provider: Groq Whisper (API-compatible with OpenAI, configurable model)
- [ ] HTTP API provider: Deepgram Nova (binary body, query params, configurable model)
- [ ] Local provider: whisper.cpp exec (configurable binary path, model path)
- [ ] Media download method on Kapso client with size limit enforcement (25MB default)
- [ ] Extract integration: audio → download → transcribe → `[voice] ` prefix → normal pipeline
- [ ] Graceful degradation: transcription failure falls back to `[audio] (mime)` with log warning
- [ ] Config section `[transcribe]` with provider, api_key, model, language, max_audio_size
- [ ] Env overrides: TRANSCRIBE_PROVIDER, TRANSCRIBE_API_KEY, TRANSCRIBE_MODEL, TRANSCRIBE_LANGUAGE
- [ ] Wiring in main.go: build Transcriber from config at startup, nil if disabled
- [ ] Table-driven tests for each provider, media download, and extract integration

### Out of Scope

- Live voice/video call handling — Pipecat territory
- Sending audio replies back to WhatsApp — text-only replies
- Transcribing video message audio tracks — future scope
- Google Cloud STT — adds heavy SDK dependency, not aligned with minimal deps convention

## Context

- Existing Go 1.22 codebase, minimal deps (gorilla/websocket, BurntSushi/toml)
- Released as v0.1.0, stable polling + webhook delivery
- `GetMediaURL(mediaID)` already exists in kapso client — download method is the missing piece
- WhatsApp voice notes are short (<2min), batch STT is the right approach
- OpenAI and Groq share the same multipart API format — can share implementation logic
- Deepgram uses a different API shape (binary body + query params)
- Local whisper.cpp needs temp file + exec + stdout capture + ffmpeg for OGG→WAV

## Constraints

- **Minimal deps**: No SDKs — all providers use standard `net/http`
- **CGO disabled**: Local provider must shell out to whisper.cpp, not link C libraries
- **Backward compatible**: Empty/missing provider config = transcription disabled, zero behavior change
- **No globals**: All state in structs, dependency-injected
- **Concurrency safe**: Transcriber implementations must be stateless and safe for concurrent use

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Two architectures (HTTP API + local) | Covers cloud-first and privacy-first deployments | — Pending |
| Configurable model per provider | Users can pick specific model versions (e.g., whisper-1, whisper-large-v3) | — Pending |
| `[voice]` prefix on transcribed text | Distinguishes transcribed audio from typed text in the pipeline | — Pending |
| Graceful fallback to `[audio] (mime)` | Transcription failure must not break message flow | — Pending |
| No Google Cloud STT | Avoids heavy SDK, inconsistent with minimal deps principle | — Pending |

---
*Last updated: 2026-03-01 after initialization*
