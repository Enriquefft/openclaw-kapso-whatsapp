package delivery

import (
	"context"
	"log"
	"sync"
	"time"
)

// Merge fans in multiple Sources with message-ID deduplication.
type Merge struct {
	Sources []Source
	seen    sync.Map
}

// Run starts all sources concurrently, deduplicates by Event.ID, and forwards
// unique events to out. It returns when all sources have finished.
func (m *Merge) Run(ctx context.Context, out chan<- Event) error {
	ch := make(chan Event, 64)

	var wg sync.WaitGroup
	for _, src := range m.Sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			if err := s.Run(ctx, ch); err != nil && ctx.Err() == nil {
				log.Printf("source error: %v", err)
			}
		}(src)
	}

	// Close ch when all sources are done.
	go func() {
		wg.Wait()
		close(ch)
	}()

	for evt := range ch {
		if _, loaded := m.seen.LoadOrStore(evt.ID, struct{}{}); loaded {
			log.Printf("merge: skipping duplicate message %s", evt.ID)
			continue
		}
		out <- evt
	}

	close(out)
	return nil
}

// StartCleanup periodically clears the dedup set to bound memory usage.
func (m *Merge) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.seen.Range(func(key, _ interface{}) bool {
				m.seen.Delete(key)
				return true
			})
		}
	}
}
