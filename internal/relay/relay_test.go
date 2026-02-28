package relay

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestConcurrentClaimsUniqueReplies verifies that when multiple relay
// goroutines race to read the same session JSONL file, each one claims a
// different assistant reply — no duplicates, no missed replies.
func TestConcurrentClaimsUniqueReplies(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	lines := ""
	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i+1) * time.Second)
		lines += fmt.Sprintf(
			`{"type":"message","timestamp":"%s","message":{"role":"assistant","stopReason":"stop","content":[{"type":"text","text":"reply-%d"}]}}`,
			ts.Format(time.RFC3339), i+1,
		) + "\n"
	}
	if err := os.WriteFile(sessionFile, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	since := base
	tracker := NewTracker()

	const goroutines = 3
	claimed := make([]string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			replies, err := getAssistantReplies(sessionFile, since)
			if err != nil {
				t.Errorf("goroutine %d: getAssistantReplies: %v", g, err)
				return
			}
			for _, r := range replies {
				if tracker.Claim(r.Key) {
					claimed[g] = r.Text
					return
				}
			}
		}()
	}

	wg.Wait()

	seen := map[string]int{}
	for g, text := range claimed {
		if text == "" {
			t.Errorf("goroutine %d got no reply", g)
			continue
		}
		seen[text]++
	}

	for text, count := range seen {
		if count > 1 {
			t.Errorf("reply %q was claimed %d times (want 1)", text, count)
		}
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique replies, got %d: %v", goroutines, len(seen), seen)
	}
}
