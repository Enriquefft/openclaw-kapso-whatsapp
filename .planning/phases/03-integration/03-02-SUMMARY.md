---
phase: 03-integration
plan: 02
subsystem: delivery
tags: [transcription, audio, whatsapp, poller, webhook, golang]

# Dependency graph
requires:
  - phase: 02-cloud-providers
    provides: transcribe.Transcriber interface and New() factory
  - phase: 03-integration
    plan: 01
    provides: local whisper.cpp provider completing the factory
provides:
  - ExtractText with Transcriber and maxAudioSize parameters
  - Audio transcription branch returning [voice] prefix on success
  - [audio] (mime) fallback on any failure with WARN log
  - Transcriber field wired through Poller and Server structs
  - main.go passes transcriber from config to all delivery sources
affects: [relay, gateway, future audio processing phases]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Interface nil guard: use typed interface field (transcribe.Transcriber) in test structs to avoid *T(nil) != nil interface pitfall"
    - "Audio transcription: waterfall pattern — GetMediaURL → DownloadMedia → Transcribe, each failure logs WARN and falls through to [audio] fallback"
    - "context.Background() for audio transcription — provider-level 30s timeout handles deadline, no separate pipeline timeout"

key-files:
  created: []
  modified:
    - internal/delivery/extract.go
    - internal/delivery/extract_test.go
    - internal/delivery/poller/poller.go
    - internal/delivery/webhook/server.go
    - cmd/kapso-whatsapp-poller/main.go

key-decisions:
  - "context.Background() passed to Transcribe — provider-level timeout (30s) is sufficient; no separate pipeline timeout needed"
  - "WARN log on each failure step (GetMediaURL, DownloadMedia, Transcribe) — silent fallback, message never lost, operator can diagnose from logs"
  - "transcribe.Transcriber interface type in test struct (not *mockTranscriber) — avoids Go interface nil pitfall where (*T)(nil) != nil interface"
  - "Fallback always calls formatMediaMessage which does its own best-effort GetMediaURL — double call accepted for simplicity; audio is already a fallback path"

patterns-established:
  - "Typed interface field in test table: use interface type for nil-able fields to avoid non-nil interface wrapping nil pointer"

requirements-completed: [TRNS-02, TRNS-03, WIRE-02, WIRE-03, INFR-02, TEST-03]

# Metrics
duration: 6min
completed: 2026-03-01
---

# Phase 3 Plan 02: Wire Transcriber into Delivery Pipeline Summary

**ExtractText widened to accept Transcriber + maxAudioSize, audio messages now transcribe to [voice] text with [audio] (mime) fallback, wired through Poller/Server/main.go**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-01T15:57:38Z
- **Completed:** 2026-03-01T16:04:00Z
- **Tasks:** 2 (TDD task with 3 commits + wiring task)
- **Files modified:** 5

## Accomplishments

- ExtractText signature widened to `(msg, client, tr, maxAudioSize)` — all non-audio types unaffected
- Audio transcription waterfall: GetMediaURL → DownloadMedia → Transcribe → `[voice] text`, fallback to `[audio] (mime)` on any failure with WARN log
- Nil transcriber preserves exact previous behavior (zero regression)
- Transcriber field added to Poller and Server structs; wired from `cfg.Transcribe` in main.go
- `_ = transcriber` placeholder removed from main.go
- 6 new test cases: success, transcription error, nil transcriber, nil audio content, media URL error, download error

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for ExtractText audio transcription** - `632cf66` (test)
2. **Task 1 GREEN: Widen ExtractText with Transcriber and audio branch** - `ea446f0` (feat)
3. **Task 2: Wire Transcriber through Poller, Server, and main.go** - `e30eebc` (feat)

_Note: TDD task has RED + GREEN commits; interface nil fix incorporated into GREEN commit._

## Files Created/Modified

- `internal/delivery/extract.go` - ExtractText widened to 4 params; audio case now has transcription waterfall with [voice] prefix and [audio] fallback
- `internal/delivery/extract_test.go` - All existing calls updated to 4 args; mockTranscriber added; 6 new audio transcription test cases
- `internal/delivery/poller/poller.go` - Transcriber and MaxAudioSize fields added; ExtractText call updated
- `internal/delivery/webhook/server.go` - Transcriber and MaxAudioSize fields added; ExtractText call updated
- `cmd/kapso-whatsapp-poller/main.go` - Placeholder removed; Transcriber and MaxAudioSize wired into Poller and Server construction

## Decisions Made

- **context.Background() for Transcribe call** — Provider-level 30s timeout is sufficient; adding a pipeline deadline would complicate the call chain without benefit
- **WARN log on each failure step** — Silent fallback so message is never lost; operator can diagnose from logs; no notification sent to WhatsApp sender
- **transcribe.Transcriber as test struct field type** — Prevents the classic Go interface nil pitfall: `(*mockTranscriber)(nil)` is a non-nil interface value; using the interface type lets `nil` remain truly nil
- **Fallback calls formatMediaMessage** — formatMediaMessage does its own best-effort GetMediaURL call, causing a double call in the failure path; accepted as a known trade-off for code simplicity in a non-critical fallback path

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Go interface nil pitfall in test struct**
- **Found during:** Task 1 GREEN phase (running tests)
- **Issue:** Test struct used `*mockTranscriber` field type — passing `nil` created `(*mockTranscriber)(nil)` which is a non-nil interface value, causing `tr != nil` check to pass and then panic on `Transcribe()` call
- **Fix:** Changed test struct field type from `*mockTranscriber` to `transcribe.Transcriber` interface — nil remains truly nil
- **Files modified:** internal/delivery/extract_test.go
- **Verification:** `nil_transcriber` test case now passes without panic
- **Committed in:** ea446f0 (Task 1 GREEN commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug in test code)
**Impact on plan:** Fix was essential for correct test behavior; no scope creep.

## Issues Encountered

The Go interface nil pitfall in test code required a fix: `*mockTranscriber(nil)` creates a non-nil interface, so the `tr != nil` guard in extract.go incorrectly entered the transcription branch and panicked. Fixed by using `transcribe.Transcriber` as the field type in the test struct, so `nil` stays nil.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Full transcription pipeline is now connected end-to-end: config → factory → delivery layer → ExtractText → audio branch
- Audio messages from WhatsApp users will produce `[voice] <transcript>` or `[audio] (mime)` depending on transcriber availability
- Phase 3 integration is complete — all delivery sources carry the Transcriber
- Remaining work (if any): local whisper.cpp provider was completed in 03-01; all providers now available

---
*Phase: 03-integration*
*Completed: 2026-03-01*
