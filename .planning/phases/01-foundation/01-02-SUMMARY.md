---
phase: 01-foundation
plan: 02
subsystem: transcribe
tags: [go, transcriber, interface, factory, media-download, tdd]

# Dependency graph
requires:
  - TranscribeConfig struct (plan 01-01)
provides:
  - Transcriber interface with Transcribe(ctx, audio, mimeType) signature
  - New(cfg TranscribeConfig) factory with provider normalization and validation
  - DownloadMedia(url, maxBytes) method on kapso.Client with io.LimitReader enforcement
  - transcribe.New() wired at startup in main.go (fatals on config error)
affects: [02-providers, 03-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TDD: RED (failing test commit) then GREEN (implementation commit)"
    - "io.LimitReader with maxBytes+1 sentinel for size enforcement without double-read"
    - "Provider normalization: strings.ToLower + strings.TrimSpace before switch"
    - "Typed nil guard: factory returns untyped nil (not typed nil pointer) for disabled state"
    - "Stub-crash pattern: cloud providers error at startup if configured before Phase 2 exists"

key-files:
  created:
    - internal/transcribe/transcribe.go
    - internal/transcribe/transcribe_test.go
    - internal/kapso/client_test.go
  modified:
    - internal/kapso/client.go
    - cmd/kapso-whatsapp-poller/main.go

key-decisions:
  - "Stub-crash pattern for cloud providers: New() errors at startup if openai/groq/deepgram configured before Phase 2 — prevents silent misconfiguration"
  - "maxBytes passed as DownloadMedia parameter (not stored on Client) to keep Client stateless re: transcription config"
  - "io.LimitReader(body, maxBytes+1) sentinel avoids reading full oversized response; len(data)>maxBytes check confirms excess"
  - "local provider uses exec.LookPath to validate binary existence at startup, not at transcription time"

requirements-completed: [TRNS-01, MEDL-01, MEDL-02, MEDL-03, MEDL-04, WIRE-01, TEST-04]

# Metrics
duration: 2min
completed: 2026-03-01
---

# Phase 1 Plan 02: Transcriber Interface, DownloadMedia, and main.go Wiring Summary

**Transcriber interface with provider-normalizing factory, DownloadMedia with io.LimitReader size enforcement, and startup wiring in main.go — delivering the core contracts Phase 2 providers implement against**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-01T13:52:22Z
- **Completed:** 2026-03-01T13:54:16Z
- **Tasks:** 3 (Tasks 1 and 2 used TDD: 2 commits each; Task 3 had 1 commit)
- **Files created:** 3, **Files modified:** 2

## Accomplishments

- Created `internal/transcribe/transcribe.go` with `Transcriber` interface and `New()` factory
- `New()` normalizes provider (lowercase + trim) before dispatch; empty provider returns (nil, nil)
- Cloud providers (openai/groq/deepgram) require API key; stub-crash at startup if configured before Phase 2
- Local provider validates binary path via `exec.LookPath` at startup; stub-crash if found but Phase 3 not implemented
- Unknown providers return descriptive error listing valid options
- Added `DownloadMedia(url string, maxBytes int64) ([]byte, error)` to `kapso.Client`
- Size enforcement via `io.LimitReader(resp.Body, maxBytes+1)` sentinel trick — no double read
- Returns X-API-Key header with client's key; error on non-200 with status code in message
- Wired `transcribe.New(cfg.Transcribe)` in `main.go` after `cfg.Validate()`; fatals on error; `_ = transcriber` until Phase 3
- Full TDD coverage: 11 table-driven subtests for factory, 5 subtests for DownloadMedia; all pass
- `go vet ./...` clean; `go build ./cmd/kapso-whatsapp-poller/` succeeds

## Task Commits

Each task was committed atomically (TDD pattern for Tasks 1 and 2):

1. **Task 1: RED - Failing tests** - `1a396e5` (test)
2. **Task 1: GREEN - Implementation** - `3e1a221` (feat)
3. **Task 2: RED - Failing tests** - `eb85d3a` (test)
4. **Task 2: GREEN - Implementation** - `a9de814` (feat)
5. **Task 3: Wire main.go** - `655cf1b` (feat)

_Note: TDD tasks have multiple commits (test then feat)_

## Files Created/Modified

- `internal/transcribe/transcribe.go` - Created: Transcriber interface + New() factory with provider normalization, validation, and stub-crash patterns
- `internal/transcribe/transcribe_test.go` - Created: 11 table-driven subtests covering all factory paths (empty, cloud with/without key, local missing binary, unknown, case normalization)
- `internal/kapso/client.go` - Modified: Added DownloadMedia method with io.LimitReader enforcement, X-API-Key header, non-200 error handling
- `internal/kapso/client_test.go` - Created: 5 subtests (under/at/over limit, API key header, non-200 status); includes rewriteTransport helper
- `cmd/kapso-whatsapp-poller/main.go` - Modified: Added transcribe import, transcribe.New() call after Validate(), fatal on error, `_ = transcriber` until Phase 3

## Decisions Made

- **Stub-crash pattern:** Cloud providers error at startup if configured before Phase 2 exists — prevents silent misconfiguration where transcription is requested but silently skipped
- **Stateless DownloadMedia:** `maxBytes` passed as parameter (not stored on `Client`) to keep `Client` stateless with respect to transcription config — caller (delivery layer) owns the limit from `cfg.Transcribe.MaxAudioSize`
- **io.LimitReader sentinel:** Read `maxBytes+1`, then check `len(data) > maxBytes` — avoids reading full oversized response while detecting overflow with a single read pass
- **Local provider validates at startup:** `exec.LookPath` runs at New() time so misconfiguration (missing binary) is caught during startup, not mid-message processing

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — interface and factory defined; providers stub-crash until Phase 2/3 implements them.

## Next Phase Readiness

- `Transcriber` interface is the contract for all Phase 2 provider implementations (openai, groq, deepgram)
- Phase 2 providers: implement `Transcriber` interface, remove "not yet implemented" stubs in `New()`
- Phase 3: replace `_ = transcriber` in `main.go` with actual delivery layer wiring (WIRE-02, WIRE-03)
- `DownloadMedia` is ready for use by the audio extraction branch in Phase 3

## Self-Check: PASSED

All files verified present. All commits verified in git log.
