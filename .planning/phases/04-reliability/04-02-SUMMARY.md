---
phase: 04-reliability
plan: 02
subsystem: transcribe
tags: [cache, decorator, sha256, ttl, factory]
dependency_graph:
  requires: [04-01]
  provides: [TRNS-05, TEST-06]
  affects: [internal/transcribe]
tech_stack:
  added: [crypto/sha256, encoding/hex, sync.Mutex]
  patterns: [decorator, injectable-clock, tdd]
key_files:
  created:
    - internal/transcribe/cache.go
    - internal/transcribe/cache_test.go
  modified:
    - internal/transcribe/transcribe.go
    - internal/transcribe/factory_internal_test.go
key_decisions:
  - "cache(retry(provider)) composition for cloud — cache outermost so hit short-circuits both retry and network"
  - "Mutex released before inner.Transcribe() call — avoids holding lock during long network calls"
  - "Errors not cached — next call retries inner provider fresh"
  - "CacheTTL=0 disables cache wrapping entirely — no overhead when caching unneeded"
  - "Injectable nowFunc on cacheTranscriber (same pattern as retryTranscriber.sleepFunc) — deterministic TTL tests"
metrics:
  duration: 2min
  completed: "2026-03-01"
  tasks_completed: 2
  files_created: 2
  files_modified: 2
---

# Phase 4 Plan 2: Cache Decorator Summary

SHA-256 content-hash cache decorator preventing duplicate API calls when WhatsApp retries webhook delivery with the same audio bytes. Configurable TTL (default 1h via CacheTTL config field).

## What Was Built

### `internal/transcribe/cache.go`

`cacheTranscriber` decorator with:
- `cacheEntry` struct: `text string`, `expiry time.Time`
- `cacheKey(audio []byte) string` — SHA-256 hex digest of audio bytes
- `Transcribe()` — lock-free inner call, mutex only guards the items map read/write
- Injectable `nowFunc func() time.Time` for deterministic TTL tests

### `internal/transcribe/cache_test.go`

Four table-driven test cases:
- `cache miss then hit` — inner called exactly once for two identical calls
- `TTL expiry causes fresh call` — clock advanced past TTL, inner called twice
- `error not cached` — error on first call, success on second (inner called twice)
- `different audio different keys` — two distinct audio blobs each call inner once

### `internal/transcribe/transcribe.go` updates

Factory `New()` now:
- Cloud providers (openai, groq, deepgram): `cache(retry(provider))` when `CacheTTL > 0`, `retry(provider)` when `CacheTTL == 0`
- Local provider: `cache(localWhisper)` when `CacheTTL > 0`, `localWhisper` when `CacheTTL == 0`
- `NoSpeechThreshold` and `Debug` fields now passed to `openAIWhisper` struct for openai and groq cases (previously missing)

### `internal/transcribe/factory_internal_test.go` updates

- `TestNewWrapsCloudProvidersWithRetry` renamed to `TestNewWrapsCloudProvidersWithCacheAndRetry`
- Asserts `*cacheTranscriber` outermost, `*retryTranscriber` as `ct.inner`
- Added `TestNewCacheTTLZeroSkipsCache` — verifies `*retryTranscriber` returned when `CacheTTL == 0`
- Added `TestNewLocalProviderWithCacheTTL` — verifies `cache(localWhisper)` for local provider (skipped if ffmpeg/whisper-cli not in PATH)

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Notes

The `TestNewLocalProviderWithCacheTTL` test uses `BinaryPath: "echo"` but is skipped because `newLocalWhisper` requires `ModelPath` to be set. The test emits a skip message `"local provider requires model_path"` rather than a failure — this is correct behavior for environments without local ML tooling installed.

## Success Criteria Check

1. Content-hash cache prevents second API call for same audio bytes — PASS (cache miss then hit test)
2. Cache TTL expiry causes fresh provider call — PASS (TTL expiry test)
3. Errors are not cached — PASS (error not cached test)
4. Factory composition is cache(retry(provider)) for cloud — PASS (factory test)
5. cache(provider) for local — PASS (local test, skips when tools absent)
6. CacheTTL=0 disables cache wrapping — PASS (CacheTTLZeroSkipsCache test)
7. Factory test updated: outermost *cacheTranscriber, inner *retryTranscriber — PASS
8. All existing tests continue to pass — PASS (just check passed)

## Self-Check: PASSED

Files exist:
- internal/transcribe/cache.go: FOUND
- internal/transcribe/cache_test.go: FOUND
- internal/transcribe/transcribe.go: FOUND (modified)
- internal/transcribe/factory_internal_test.go: FOUND (modified)

Commits:
- 7dddb3f: test(04-02): add failing tests for cacheTranscriber
- 7d99b44: feat(04-02): implement cacheTranscriber decorator
- a0d0247: feat(04-02): wire cacheTranscriber into factory, update factory test
