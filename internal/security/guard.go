package security

import (
	"strings"
	"sync"
	"time"

	"github.com/hybridz/openclaw-kapso-whatsapp/internal/config"
)

// Verdict represents the outcome of a guard check.
type Verdict int

const (
	Allow       Verdict = iota
	Deny
	RateLimited
)

// bucket tracks rate limit state for a single sender.
type bucket struct {
	tokens    int
	windowEnd time.Time
}

// Guard enforces sender allowlist, rate limiting, role resolution, and session isolation.
type Guard struct {
	mode        string
	phoneTo     map[string]string // normalized phone → role
	defaultRole string
	denyMessage string
	rateLimit   int
	rateWindow  time.Duration
	isolate     bool
	now         func() time.Time
	mu          sync.Mutex
	buckets     map[string]*bucket
}

// New creates a Guard from the security config. It inverts the role→[]phones
// map into a phone→role lookup for O(1) checks.
func New(cfg config.SecurityConfig) *Guard {
	phoneTo := make(map[string]string)
	for role, phones := range cfg.Roles {
		for _, phone := range phones {
			n := normalize(phone)
			if _, exists := phoneTo[n]; !exists {
				phoneTo[n] = role
			}
		}
	}

	return &Guard{
		mode:        cfg.Mode,
		phoneTo:     phoneTo,
		defaultRole: cfg.DefaultRole,
		denyMessage: cfg.DenyMessage,
		rateLimit:   cfg.RateLimit,
		rateWindow:  time.Duration(cfg.RateWindow) * time.Second,
		isolate:     cfg.SessionIsolation,
		now:         time.Now,
		buckets:     make(map[string]*bucket),
	}
}

// Check returns Allow, Deny, or RateLimited for the given sender phone number.
func (g *Guard) Check(from string) Verdict {
	n := normalize(from)

	if g.mode == "allowlist" {
		if _, ok := g.phoneTo[n]; !ok {
			return Deny
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	b, ok := g.buckets[n]
	if !ok || now.After(b.windowEnd) {
		g.buckets[n] = &bucket{
			tokens:    g.rateLimit - 1,
			windowEnd: now.Add(g.rateWindow),
		}
		return Allow
	}

	if b.tokens <= 0 {
		return RateLimited
	}
	b.tokens--
	return Allow
}

// Role returns the sender's role. In allowlist mode, returns the mapped role.
// In open mode, returns the mapped role if the sender is in the roles map,
// otherwise returns the default role.
func (g *Guard) Role(from string) string {
	n := normalize(from)
	if role, ok := g.phoneTo[n]; ok {
		return role
	}
	return g.defaultRole
}

// DenyMessage returns the configured denial message.
func (g *Guard) DenyMessage() string {
	return g.denyMessage
}

// SessionKey returns a per-sender session key if isolation is enabled,
// otherwise returns the base key unchanged.
func (g *Guard) SessionKey(baseKey, from string) string {
	if !g.isolate {
		return baseKey
	}
	n := normalize(from)
	// Strip the leading + for the session key suffix.
	suffix := strings.TrimPrefix(n, "+")
	return baseKey + "-wa-" + suffix
}

// normalize strips all characters except digits and a leading +.
func normalize(phone string) string {
	if phone == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(phone))

	for i, r := range phone {
		if r == '+' && i == 0 {
			b.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}

	return b.String()
}
