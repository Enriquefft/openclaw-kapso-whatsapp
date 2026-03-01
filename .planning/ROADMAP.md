# Roadmap: Kapso WhatsApp — Voice Transcription

## Overview

This milestone adds voice transcription to the existing WhatsApp-OpenClaw bridge. Incoming audio messages are intercepted, downloaded, transcribed via a configurable provider (Groq, OpenAI, Deepgram, or local whisper.cpp), and injected into the relay pipeline as `[voice] <transcript>` — transparently, with graceful fallback if transcription fails. Four phases: foundation contracts, cloud providers, full pipeline integration, and reliability hardening.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Config section, Transcriber interface, factory, and media download
- [x] **Phase 2: Cloud Providers** - OpenAI/Groq shared implementation and Deepgram provider (completed 2026-03-01)
- [x] **Phase 3: Integration** - Local whisper.cpp provider, pipeline wiring, and end-to-end flow (completed 2026-03-01)
- [ ] **Phase 4: Reliability** - Retry, caching, no_speech_prob guard, debug logging, and full test coverage

## Phase Details

### Phase 1: Foundation
**Goal**: The contract for transcription is defined and audio bytes can be fetched safely
**Depends on**: Nothing (first phase)
**Requirements**: CONF-01, CONF-02, CONF-03, CONF-04, CONF-05, TRNS-01, MEDL-01, MEDL-02, MEDL-03, MEDL-04, WIRE-01, TEST-04
**Success Criteria** (what must be TRUE):
  1. `config.TranscribeConfig` is parsed from `[transcribe]` TOML and env overrides with correct 3-tier precedence
  2. Empty or missing provider config results in transcription disabled with zero behavior change to existing message flow
  3. `transcribe.Transcriber` interface and `New(cfg)` factory exist — unknown provider string returns an error at startup
  4. `kapso.Client.DownloadMedia` fetches audio bytes and rejects responses exceeding the configured size limit
  5. Media download test passes: size-limit enforcement verified against a mock HTTP server
**Plans:** 2 plans

Plans:
- [x] 01-01-PLAN.md — TranscribeConfig struct, TOML/env parsing, defaults, and validation
- [x] 01-02-PLAN.md — Transcriber interface, factory, DownloadMedia method, and main.go wiring

### Phase 2: Cloud Providers
**Goal**: Audio messages can be transcribed via Groq, OpenAI, or Deepgram using only stdlib HTTP
**Depends on**: Phase 1
**Requirements**: PROV-01, PROV-02, PROV-03, PROV-04, INFR-03, TEST-01, TEST-05
**Success Criteria** (what must be TRUE):
  1. Groq and OpenAI providers share one implementation (configurable `BaseURL`) with no duplicated HTTP logic
  2. Deepgram provider posts binary audio body with correct Content-Type and query params, parses nested response path
  3. MIME normalisation helper maps all OGG/Opus variants to `audio/ogg` before any provider call
  4. Table-driven tests for each provider pass against mock HTTP servers, including MIME boundary construction
  5. Retry logic test passes: 429/5xx triggers backoff, success after retry, exhausted retries returns error
**Plans:** 2/2 plans complete

Plans:
- [x] 02-01-PLAN.md — OpenAI/Groq shared provider with MIME normalization, table-driven tests, and factory wiring
- [x] 02-02-PLAN.md — Deepgram provider, retry wrapper infrastructure, and factory retry wrapping

### Phase 3: Integration
**Goal**: Audio messages flow end-to-end from WhatsApp through transcription into the relay pipeline
**Depends on**: Phase 2
**Requirements**: TRNS-02, TRNS-03, LOCL-01, LOCL-02, LOCL-03, LOCL-04, WIRE-02, WIRE-03, INFR-02, TEST-02, TEST-03
**Success Criteria** (what must be TRUE):
  1. A WhatsApp audio message results in `[voice] <transcript>` text entering the relay pipeline, identical to typed text
  2. When transcription fails for any reason, pipeline receives `[audio] (mime)` with a WARN log — message is never lost
  3. Local whisper.cpp provider converts OGG to WAV via ffmpeg, runs whisper-cli, and cleans up temp files on completion and context cancellation
  4. `delivery.ExtractText` accepts a nil Transcriber and preserves current behavior unchanged for all non-audio messages
  5. `main.go` builds Transcriber from config at startup — nil if provider is unconfigured
**Plans:** 2/2 plans complete

Plans:
- [x] 03-01-PLAN.md — Local whisper.cpp provider with ffmpeg conversion, temp file cleanup, and factory wiring
- [ ] 03-02-PLAN.md — ExtractText pipeline wiring, audio transcription branch, and main.go startup integration

### Phase 4: Reliability
**Goal**: Transcription is hardened against transient failures, duplicate billing, and silent hallucinations
**Depends on**: Phase 3
**Requirements**: TRNS-04, TRNS-05, INFR-01, INFR-04, TEST-06
**Success Criteria** (what must be TRUE):
  1. Retry wrapper retries on 429/5xx with exponential backoff (3 attempts, 1s base, 2x factor, jitter) and respects context cancellation
  2. Content-hash cache returns cached transcript on second call with same audio bytes — no second API call made
  3. Cache TTL expiry causes a cache miss and a fresh provider call
  4. High `no_speech_prob` (above configured threshold, default 0.85) falls back to `[audio]` instead of emitting a hallucinated transcript
  5. Debug logging emits `avg_logprob`, `no_speech_prob`, and detected language from verbose_json responses at debug level
**Plans**: TBD

Plans:
- [ ] 04-01: Retry wrapper, content-hash cache, no_speech_prob guard, and debug logging

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 2/2 | Complete | 2026-03-01 |
| 2. Cloud Providers | 2/2 | Complete   | 2026-03-01 |
| 3. Integration | 2/2 | Complete   | 2026-03-01 |
| 4. Reliability | 0/1 | Not started | - |
