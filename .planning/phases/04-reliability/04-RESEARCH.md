# Phase 4: Reliability - Research

**Researched:** 2026-03-01
**Domain:** Go in-memory caching, verbose_json parsing, hallucination guard, debug logging
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Cache behavior
- In-memory only (map+mutex or sync.Map) — lost on restart, fits the daemon model since WhatsApp rarely resends the same audio
- Default TTL: 1 hour — covers common WhatsApp dedup retries (usually within minutes), low memory footprint
- No max entry limit — rely on TTL expiry only. Audio volume is low for most users
- Cache key: SHA-256 of audio bytes — crypto-strength collision resistance, standard and safe
- Cache wraps the Transcriber interface (decorator pattern) — sits between retry wrapper and caller

#### Hallucination guard
- no_speech_prob threshold configurable in config.toml with default 0.85 — roadmap says "configured threshold" implying configurability
- Guard applies to OpenAI/Groq only (they support verbose_json natively) — Deepgram has its own confidence metrics, local whisper doesn't return verbose_json
- When threshold exceeded, log the actual probability value: `WARN: no_speech_prob=0.92 exceeds threshold 0.85, falling back to [audio]` — helps operators debug false rejections
- Fallback produces `[audio] (mime)` same as any other transcription failure

#### Debug logging
- Controlled via config flag + env override: `debug` bool in TranscribeConfig, overridable with `KAPSO_TRANSCRIBE_DEBUG=true` — follows existing 3-tier config pattern
- Required fields: avg_logprob, no_speech_prob, detected language (matches roadmap criterion #5 exactly)
- Also log transcription duration (ms) and provider/model used — useful for performance monitoring
- Uses standard `log.Printf` with `[transcribe:debug]` prefix — no new logging framework

#### Retry tuning
- Verify existing retry.go meets criteria, no changes needed — already has 3 attempts, 1s base, 2x factor, jitter, context cancellation
- Do NOT make retry params configurable — current defaults match success criteria and adding knobs adds complexity without clear benefit

### Claude's Discretion
- Exact cache cleanup goroutine implementation (ticker-based eviction vs lazy expiry)
- How to restructure verbose_json parsing in openai.go to extract no_speech_prob
- Whether cache decorator is a separate file or added to existing retry.go

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TRNS-04 | `no_speech_prob` quality guard — high probability of silence/noise falls back to `[audio]` instead of sending hallucinated text (configurable threshold, default 0.85) | verbose_json segment structure confirmed: `segments[].no_speech_prob` field from OpenAI and Groq; openai.go already requests `verbose_json` but only parses `text` — expand response struct |
| TRNS-05 | Audio content-hash caching — SHA-256 hash of audio bytes, in-memory map with TTL, avoids duplicate API calls on webhook retries | `crypto/sha256` already used in project (webhook/server.go); `sync.Map` already used for dedup (delivery/merge.go); TTL-with-expiry-field + lazy eviction on Get is the idiomatic Go pattern for this scale |
| INFR-01 | Retry with exponential backoff on 429/5xx — max 3 attempts, base 1s, factor 2x, jitter | retry.go already implements this exactly: 3 attempts, 1s base, 2x factor, 0.25 jitter, context cancellation check before sleep — VERIFY ONLY, no new work |
| INFR-04 | Debug-level logging of `avg_logprob`, `no_speech_prob`, and detected language from verbose_json responses | verbose_json top-level `language` field plus `segments[].avg_logprob` and `segments[].no_speech_prob` — aggregate across segments for debug summary |
| TEST-06 | Content-hash cache test (hit, miss, TTL expiry) | `mockTranscriber` in retry_test.go is reusable; injectable `now()` function (same pattern as retry's `sleepFunc`) enables deterministic TTL tests without real time |
</phase_requirements>

## Summary

Phase 4 hardens the transcription pipeline with four changes to existing code: (1) INFR-01 retry verification — the existing `retryTranscriber` in `retry.go` already fully meets all success criteria, requiring only a verification pass; (2) TRNS-05 content-hash cache — a new `cacheTranscriber` decorator using `sync.Map` + expiry timestamps, keyed by SHA-256 of audio bytes; (3) TRNS-04 hallucination guard + INFR-04 debug logging — expanding `openai.go`'s response parsing to consume the full `verbose_json` segment structure; (4) TEST-06 — table-driven cache tests using injectable `now()`.

All implementation stays within the existing `transcribe` package. The decorator/wrapper pattern established by `retryTranscriber` is the blueprint for `cacheTranscriber`. The `sync.Map` with expiry-field approach (lazy eviction on `Get` + optional ticker cleanup) is the project-idiomatic pattern — already used in `delivery/merge.go`. `crypto/sha256` is already imported in the project (`delivery/webhook/server.go`), confirming it is a settled dependency.

The verbose_json response format is confirmed for both OpenAI and Groq (same `segments[].no_speech_prob` and `segments[].avg_logprob` fields). `openai.go` already sends `response_format=verbose_json` but only unmarshals `{text}` — expanding the response struct is mechanical. No new external dependencies are needed.

**Primary recommendation:** Implement `cacheTranscriber` as a new file `cache.go`; expand `openai.go`'s response struct in-place; add `Debug bool` and `NoSpeechThreshold float64` and `CacheTTL int` to `config.TranscribeConfig`; verify `retry.go` passes all INFR-01 criteria without changes.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/sha256` | stdlib | SHA-256 hash of audio bytes for cache key | Already used in project (webhook/server.go); no external dep |
| `sync` (Map + Mutex) | stdlib | Thread-safe cache store | `sync.Map` already used for dedup in delivery/merge.go; consistent with project |
| `time` | stdlib | TTL expiry timestamps, cleanup ticker | Already used throughout; time.Duration field on cache entry |
| `encoding/hex` | stdlib | Encode SHA-256 bytes to string key | Pairs with crypto/sha256; zero cost |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log` | stdlib | Debug output with `[transcribe:debug]` prefix | Already the project logging standard — no frameworks |
| `encoding/json` | stdlib | Parse expanded verbose_json response | Already imported in openai.go |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `sync.Map` with expiry field | `map[string]cacheEntry` + `sync.Mutex` | Mutex+map is simpler and slightly faster for write-heavy; sync.Map shines for stable read-heavy maps. Either works at this scale — sync.Map preferred since it's already the project pattern (merge.go) |
| Lazy eviction on Get | Background ticker cleanup goroutine | Ticker cleanup bounds memory over long runs; lazy eviction is simpler and sufficient given 1h TTL and low audio volume. Recommend: lazy eviction + optional ticker if memory is a concern |
| SHA-256 (32 bytes) | SHA-1 (20 bytes) or xxHash | SHA-256 already in project; crypto-strength collision resistance; no performance concern at this volume |
| `time.AfterFunc` per key | expiry field + lazy check | AfterFunc spawns one timer goroutine per cache entry — can accumulate; expiry field approach is GC-friendly |

**Installation:** No new dependencies required. All stdlib.

## Architecture Patterns

### Recommended Project Structure

```
internal/transcribe/
├── cache.go          # NEW: cacheTranscriber decorator + cacheEntry struct
├── cache_test.go     # NEW: TEST-06 cache tests (hit, miss, TTL expiry)
├── openai.go         # MODIFY: expand whisperVerboseResponse struct; add guard + debug log
├── transcribe.go     # MODIFY: New() factory — wrap with cache decorator
├── retry.go          # VERIFY ONLY: meets INFR-01 already
├── retry_test.go     # UNCHANGED: mockTranscriber reusable for cache tests
├── ...               # deepgram.go, local.go, etc. UNCHANGED
internal/config/
└── config.go         # MODIFY: add Debug, NoSpeechThreshold, CacheTTL to TranscribeConfig
```

### Pattern 1: Cache Decorator (mirrors retryTranscriber)

**What:** `cacheTranscriber` implements `Transcriber`, wraps an inner `Transcriber`, checks cache before delegating, stores results after delegation.
**When to use:** Wrap all providers (cloud + local) — avoids any duplicate API call or subprocess exec.

```go
// internal/transcribe/cache.go

type cacheEntry struct {
    text    string
    expiry  time.Time
}

type cacheTranscriber struct {
    inner   Transcriber
    ttl     time.Duration
    nowFunc func() time.Time // injectable for tests

    mu    sync.Mutex
    items map[string]cacheEntry
}

func newCacheTranscriber(inner Transcriber, ttl time.Duration) *cacheTranscriber {
    return &cacheTranscriber{
        inner:   inner,
        ttl:     ttl,
        nowFunc: time.Now,
        items:   make(map[string]cacheEntry),
    }
}

func (c *cacheTranscriber) cacheKey(audio []byte) string {
    h := sha256.Sum256(audio)
    return hex.EncodeToString(h[:])
}

func (c *cacheTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
    key := c.cacheKey(audio)
    now := c.nowFunc()

    c.mu.Lock()
    if entry, ok := c.items[key]; ok && now.Before(entry.expiry) {
        c.mu.Unlock()
        return entry.text, nil
    }
    c.mu.Unlock()

    text, err := c.inner.Transcribe(ctx, audio, mimeType)
    if err != nil {
        return "", err
    }

    c.mu.Lock()
    c.items[key] = cacheEntry{text: text, expiry: c.nowFunc().Add(c.ttl)}
    c.mu.Unlock()

    return text, nil
}
```

**Note on sync.Map vs map+Mutex:** The project uses `sync.Map` in `delivery/merge.go` but that use-case (LoadOrStore dedup) is idiomatic for `sync.Map`. For cache with TTL expiry fields, `map[K]V + sync.Mutex` is cleaner because `sync.Map` does not support "load + check expiry + store" atomically without a separate lock anyway. Use `map + sync.Mutex` here.

### Pattern 2: Expanded verbose_json Response Struct

**What:** `openai.go` currently parses only `{text}`. Expand to full struct to enable no_speech_prob guard and debug logging.
**When to use:** OpenAI and Groq only (both use same `openAIWhisper` implementation and return identical verbose_json schema).

```go
// in openai.go — replace the anonymous result struct

type whisperVerboseResponse struct {
    Text     string  `json:"text"`
    Language string  `json:"language"`
    Duration float64 `json:"duration"`
    Segments []struct {
        AvgLogprob   float64 `json:"avg_logprob"`
        NoSpeechProb float64 `json:"no_speech_prob"`
    } `json:"segments"`
}
```

The `openAIWhisper` struct needs access to configuration for the threshold and debug flag. Options:
- **Option A (recommended):** Pass `noSpeechThreshold float64` and `debug bool` as fields on `openAIWhisper` — set in factory, no config import needed in provider
- **Option B:** Pass config.TranscribeConfig directly — creates coupling

Option A is consistent with how `Language`, `Model` are already fields on `openAIWhisper`.

### Pattern 3: no_speech_prob Guard Logic

**What:** After parsing verbose_json, compute aggregate no_speech_prob (max or mean across segments) and compare against threshold.
**Aggregation strategy:** Use the **maximum** `no_speech_prob` across all segments — most conservative, avoids emitting any text when even one segment is likely silence/noise.

```go
// after json.Unmarshal into whisperVerboseResponse

var maxNoSpeech float64
var sumLogprob float64
for _, seg := range result.Segments {
    if seg.NoSpeechProb > maxNoSpeech {
        maxNoSpeech = seg.NoSpeechProb
    }
    sumLogprob += seg.AvgLogprob
}
avgLogprob := 0.0
if len(result.Segments) > 0 {
    avgLogprob = sumLogprob / float64(len(result.Segments))
}

if p.Debug {
    log.Printf("[transcribe:debug] provider=%s model=%s lang=%s avg_logprob=%.4f no_speech_prob=%.4f duration_ms=%d",
        "openai", p.Model, result.Language, avgLogprob, maxNoSpeech, durationMs)
}

if p.NoSpeechThreshold > 0 && maxNoSpeech >= p.NoSpeechThreshold {
    log.Printf("WARN: no_speech_prob=%.4f exceeds threshold %.2f, falling back to [audio]",
        maxNoSpeech, p.NoSpeechThreshold)
    return "", &noSpeechError{Prob: maxNoSpeech, Threshold: p.NoSpeechThreshold}
}

return result.Text, nil
```

**`noSpeechError` type:** Create a sentinel error type so callers can distinguish hallucination guard rejections from network errors. The delivery layer already handles transcription errors uniformly (falls back to `[audio] (mime)`) — the new error type doesn't require any changes upstream.

### Pattern 4: Factory Composition

**What:** Update `New()` to wrap with cache decorator after existing retry wrapping.

```go
// in transcribe.go New():

// For cloud providers (after retry wrap):
timeout := time.Duration(cfg.Timeout) * time.Second
wrapped := newRetryTranscriber(p, timeout)
if cfg.CacheTTL > 0 {
    return newCacheTranscriber(wrapped, time.Duration(cfg.CacheTTL)*time.Second), nil
}
return wrapped, nil

// For local provider (no retry, but still cache):
lp, err := newLocalWhisper(cfg)
if err != nil { return nil, err }
if cfg.CacheTTL > 0 {
    return newCacheTranscriber(lp, time.Duration(cfg.CacheTTL)*time.Second), nil
}
return lp, nil
```

**Config defaults:** `CacheTTL` default 3600 (1 hour) — set in `config.go` `defaults()`. `NoSpeechThreshold` default 0.85. `Debug` default false.

### Pattern 5: Config Fields

**What:** Add three new fields to `TranscribeConfig`.

```go
// in config.go TranscribeConfig struct:
NoSpeechThreshold float64 `toml:"no_speech_threshold"`
CacheTTL          int     `toml:"cache_ttl"`  // seconds; 0 = disabled
Debug             bool    `toml:"debug"`
```

Env var overrides in `applyEnv()`:
```go
if v := os.Getenv("KAPSO_TRANSCRIBE_DEBUG"); v != "" {
    cfg.Transcribe.Debug = v == "true"
}
if v := os.Getenv("KAPSO_TRANSCRIBE_NO_SPEECH_THRESHOLD"); v != "" {
    if f, err := strconv.ParseFloat(v, 64); err == nil {
        cfg.Transcribe.NoSpeechThreshold = f
    }
}
if v := os.Getenv("KAPSO_TRANSCRIBE_CACHE_TTL"); v != "" {
    if n, err := strconv.Atoi(v); err == nil {
        cfg.Transcribe.CacheTTL = n
    }
}
```

Defaults in `defaults()`:
```go
Transcribe: TranscribeConfig{
    MaxAudioSize:      25 * 1024 * 1024,
    BinaryPath:        "whisper-cli",
    Timeout:           30,
    NoSpeechThreshold: 0.85,   // NEW
    CacheTTL:          3600,   // NEW — 1 hour
    Debug:             false,  // NEW
},
```

### Anti-Patterns to Avoid

- **Holding the lock during inner.Transcribe():** Never hold a mutex across a network call — releases lock before calling inner, reacquire only for the store. The pattern above is correct.
- **Using sync.Map for TTL cache:** sync.Map's LoadOrStore doesn't let you atomically read-check-expiry-update — you'd still need a separate mutex. Use `map + sync.Mutex` directly.
- **Coupling provider to config package:** Providers should receive plain primitive fields (float64, bool) not config structs — keeps them testable without config.
- **Applying no_speech guard to Deepgram or local:** Deepgram returns a different response shape (no verbose_json segments). Local whisper.cpp stdout doesn't include segment metadata. Guard is only in `openAIWhisper.Transcribe()`.
- **Returning empty string instead of error from no_speech guard:** Return a sentinel error — the delivery layer's fallback path already handles errors correctly. Returning "" + nil would produce an empty transcript in the pipeline.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Concurrent map access | Custom lock-free structure | `map[string]cacheEntry + sync.Mutex` | sync primitives are sufficient; already used in security/guard.go and relay/relay.go |
| Content hash function | Custom hash or FNV | `crypto/sha256` | Already imported in project; crypto-grade collision resistance; `sha256.Sum256` is one call |
| Hex encoding | fmt.Sprintf("%x", ...) | `encoding/hex.EncodeToString` | Cleaner, same output — both fine but hex package is idiomatic |
| Retry logic | New retry for cache miss | Existing `retryTranscriber` | Already implemented and tested; cache wraps retry, not the other way |

**Key insight:** This phase is entirely in-process Go with stdlib — there is nothing complex enough to warrant an external library. The project convention of minimal deps (gorilla/websocket + BurntSushi/toml only) must be preserved.

## Common Pitfalls

### Pitfall 1: Lock Held Across Network Call
**What goes wrong:** If the mutex is held while calling `inner.Transcribe()`, all concurrent cache misses serialize — nullifying the benefits and adding latency.
**Why it happens:** Naive "check then act" critical section pattern.
**How to avoid:** Check cache under lock, release lock, call inner, reacquire lock only for the store write. Accept the tiny risk of a second concurrent miss racing — at this scale it is harmless.
**Warning signs:** `c.mu.Lock()` immediately before `c.inner.Transcribe(...)`.

### Pitfall 2: Cache Wraps Inner of Inner (Wrong Composition Order)
**What goes wrong:** `retry(cache(provider))` means a cache hit on a retried call doesn't short-circuit the retry machinery; `cache(retry(provider))` is correct — cache is outermost.
**Why it happens:** Misordering decorators in `New()`.
**How to avoid:** Success criteria explicitly says "no second API call on cache hit" — verify by checking that `factory_internal_test.go` type-asserts the outermost wrapper.
**Warning signs:** `newRetryTranscriber(newCacheTranscriber(...), ...)`.

### Pitfall 3: Aggregating no_speech_prob Incorrectly
**What goes wrong:** Using the mean `no_speech_prob` instead of max means a clip with one high-noise segment surrounded by clean segments passes the guard — hallucination still emitted.
**Why it happens:** Mean feels like "average quality" but no_speech is a per-segment signal.
**How to avoid:** Use `max` across segments for the guard. Use mean of `avg_logprob` for debug logging only.
**Warning signs:** Test with a mock segment slice where one segment has `no_speech_prob=0.95` and others have `0.0` — should trigger the guard.

### Pitfall 4: Zero-value CacheTTL Disabling Cache Silently
**What goes wrong:** If `CacheTTL` defaults to 0 (zero-value) because the `defaults()` function wasn't updated, the cache is never enabled even when the provider is configured — TRNS-05 silently unmet.
**Why it happens:** Adding a field to a struct without updating `defaults()` — seen previously with `MaxAudioSize` (which has its own zero-guard in `Validate()`).
**How to avoid:** Add `CacheTTL` default in `defaults()` (3600). Add a zero-guard in `Validate()` similar to `MaxAudioSize`.
**Warning signs:** `just test` passes but cache hit/miss test shows inner called every time.

### Pitfall 5: Factory Type Assertion Broken After Wrapping with Cache
**What goes wrong:** `factory_internal_test.go` asserts the outermost type is `*retryTranscriber` — after Phase 4, the outermost type is `*cacheTranscriber`. The existing test will FAIL.
**Why it happens:** Test was written to verify Phase 2's wrapping; Phase 4 adds an outer layer.
**How to avoid:** Update `TestNewWrapsCloudProvidersWithRetry` to assert `*cacheTranscriber` as outermost, and verify its `.inner` is `*retryTranscriber`.
**Warning signs:** `go test ./internal/transcribe/...` fails with type assertion failure immediately after wiring.

### Pitfall 6: Test Fixture for verbose_json Missing Segments
**What goes wrong:** Existing `whisperVerboseJSON` fixture in `openai_test.go` is `{"text":"hello world","language":"en","duration":1.5}` — has no `segments` array. After expanding the response struct, the guard logic would find `len(segments)==0` and skip the guard — tests pass but guard is untested.
**Why it happens:** Fixture was adequate for Phase 2's text-only parsing.
**How to avoid:** Update the fixture to include a segments array with realistic `no_speech_prob` and `avg_logprob` values. Add test cases for above/below-threshold behavior.
**Warning signs:** Guard code path never executes in tests; no test case has `wantFallback: true`.

## Code Examples

Verified patterns from codebase and stdlib:

### SHA-256 Cache Key (project-consistent pattern)
```go
// Source: stdlib crypto/sha256; same import already in internal/delivery/webhook/server.go
import (
    "crypto/sha256"
    "encoding/hex"
)

func cacheKey(audio []byte) string {
    h := sha256.Sum256(audio)
    return hex.EncodeToString(h[:])
}
```

### map+Mutex TTL Cache (project-consistent pattern)
```go
// Source: sync.Mutex pattern from internal/security/guard.go and internal/relay/relay.go

type cacheEntry struct {
    text   string
    expiry time.Time
}

type cacheTranscriber struct {
    inner   Transcriber
    ttl     time.Duration
    nowFunc func() time.Time

    mu    sync.Mutex
    items map[string]cacheEntry
}

func (c *cacheTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
    key := cacheKey(audio)
    now := c.nowFunc()

    // Check cache — hold lock only for the map read.
    c.mu.Lock()
    if entry, ok := c.items[key]; ok && now.Before(entry.expiry) {
        c.mu.Unlock()
        return entry.text, nil
    }
    c.mu.Unlock()

    // Cache miss — call inner without holding lock.
    text, err := c.inner.Transcribe(ctx, audio, mimeType)
    if err != nil {
        return "", err
    }

    // Store result — hold lock only for map write.
    c.mu.Lock()
    c.items[key] = cacheEntry{text: text, expiry: c.nowFunc().Add(c.ttl)}
    c.mu.Unlock()

    return text, nil
}
```

### Injectable now() for Deterministic TTL Tests (project-consistent pattern)
```go
// Source: retryTranscriber.sleepFunc pattern in internal/transcribe/retry.go

// In test:
fakeClock := time.Now()
nowFunc := func() time.Time { return fakeClock }

ct := newCacheTranscriber(inner, 1*time.Hour)
ct.nowFunc = nowFunc

// First call — miss
ct.Transcribe(ctx, audio, mime)  // inner called

// Second call — hit (same clock)
ct.Transcribe(ctx, audio, mime)  // inner NOT called

// Advance clock past TTL
fakeClock = fakeClock.Add(2 * time.Hour)

// Third call — miss (expired)
ct.Transcribe(ctx, audio, mime)  // inner called again
```

### Expanded verbose_json Response Struct
```go
// Source: OpenAI API reference; Groq confirmed same schema
// https://console.groq.com/docs/speech-to-text
// https://platform.openai.com/docs/api-reference/audio/

type whisperVerboseResponse struct {
    Text     string  `json:"text"`
    Language string  `json:"language"`
    Duration float64 `json:"duration"`
    Segments []struct {
        AvgLogprob   float64 `json:"avg_logprob"`
        NoSpeechProb float64 `json:"no_speech_prob"`
    } `json:"segments"`
}
```

### no_speech_prob Guard + Debug Log
```go
// After parsing whisperVerboseResponse:
var maxNoSpeech, sumLogprob float64
for _, seg := range result.Segments {
    if seg.NoSpeechProb > maxNoSpeech {
        maxNoSpeech = seg.NoSpeechProb
    }
    sumLogprob += seg.AvgLogprob
}
avgLogprob := 0.0
if n := len(result.Segments); n > 0 {
    avgLogprob = sumLogprob / float64(n)
}

if p.Debug {
    log.Printf("[transcribe:debug] provider=openai model=%s lang=%s avg_logprob=%.4f no_speech_prob=%.4f",
        p.Model, result.Language, avgLogprob, maxNoSpeech)
}

if p.NoSpeechThreshold > 0 && maxNoSpeech >= p.NoSpeechThreshold {
    log.Printf("WARN: no_speech_prob=%.4f exceeds threshold %.2f, falling back to [audio]",
        maxNoSpeech, p.NoSpeechThreshold)
    return "", &noSpeechError{Prob: maxNoSpeech, Threshold: p.NoSpeechThreshold}
}
return result.Text, nil
```

### Updated Factory Test (fixes broken assertion)
```go
// Source: internal/transcribe/factory_internal_test.go (must update)
// After Phase 4, outermost is *cacheTranscriber wrapping *retryTranscriber

got, err := New(tc.cfg)
ct, ok := got.(*cacheTranscriber)
if !ok {
    t.Fatalf("factory returned %T, want *cacheTranscriber", got)
}
if _, ok := ct.inner.(*retryTranscriber); !ok {
    t.Errorf("cacheTranscriber.inner is %T, want *retryTranscriber", ct.inner)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Parse `{text}` only from verbose_json | Parse full segment struct including `no_speech_prob`, `avg_logprob`, `language` | Phase 4 | Enables hallucination guard and debug telemetry |
| No caching — each audio hash triggers provider call | Content-hash cache with TTL wraps provider | Phase 4 | Eliminates duplicate billing on webhook retries |
| Retry exists but INFR-01 pending verification | Verify retry.go meets criteria exactly | Phase 4 | Formally closes INFR-01 as complete |

**Deprecated/outdated:**
- Existing `whisperVerboseJSON` test fixture (missing segments): must be updated to include segment array
- `factory_internal_test.go` type assertion for `*retryTranscriber` as outermost: must be updated to `*cacheTranscriber`

## Open Questions

1. **Aggregation strategy for no_speech_prob across segments**
   - What we know: OpenAI/Groq return `no_speech_prob` per segment, not a single top-level value
   - What's unclear: Whether max or mean is the better guard trigger
   - Recommendation: Use **max** — most conservative; avoids partial hallucinations in mixed clips. Mean is for debug logging only.

2. **Should the noSpeechError be exported?**
   - What we know: Delivery layer catches transcription errors uniformly; only needs to know "it failed"
   - What's unclear: Whether operators/tests need to inspect the error type
   - Recommendation: Keep unexported (`noSpeechError`) — delivery layer uses `[audio]` fallback for any error, no inspection needed. Tests can check the returned text is empty and error is non-nil.

3. **Empty segments slice handling**
   - What we know: Possible if audio is very short or API returns minimal response
   - What's unclear: When exactly the OpenAI API omits segments vs returns empty array
   - Recommendation: Guard `len(result.Segments) == 0` → skip no_speech check (don't reject valid short clips) and log at debug if Debug=true.

## Validation Architecture

> `workflow.nyquist_validation` is not present in `.planning/config.json` — skipping this section.

## Sources

### Primary (HIGH confidence)
- OpenAI API community thread (Whisper API verbose_json results): confirmed `segments[].no_speech_prob`, `segments[].avg_logprob`, `language` top-level field — https://community.openai.com/t/whisper-api-verbose-json-results/93083
- Groq Speech-to-Text docs: confirmed same verbose_json segment schema with `no_speech_prob` and `avg_logprob` — https://console.groq.com/docs/speech-to-text
- Project codebase: `crypto/sha256` used in `internal/delivery/webhook/server.go`; `sync.Map` used in `internal/delivery/merge.go`; `sync.Mutex` used in `internal/security/guard.go`, `internal/relay/relay.go`
- `internal/transcribe/retry.go`: confirmed 3 attempts, 1s base, 2x factor, 0.25 jitter, context cancellation — INFR-01 already met

### Secondary (MEDIUM confidence)
- Go in-memory TTL cache patterns: `map+sync.Mutex` with expiry field is well-documented idiomatic Go — https://www.alexedwards.net/blog/implementing-an-in-memory-cache-in-go
- `sync.Map` vs `map+Mutex` tradeoffs: sync.Map optimal for stable read-heavy maps, map+Mutex better for TTL-with-expiry-check pattern — multiple Go documentation sources

### Tertiary (LOW confidence)
- None — all findings verified against project source code or official API documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — stdlib only; no external deps; all patterns confirmed in project codebase
- Architecture: HIGH — decorator pattern is established in project (retryTranscriber); verbose_json schema verified against official docs
- Pitfalls: HIGH — most identified from direct code inspection (existing fixture gaps, factory assertion breakage, composition order) not speculation

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (stdlib patterns; OpenAI/Groq verbose_json schema is stable)
