package security

import (
	"strings"
	"sync"
	"time"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/config"
)

// Verdict represents the outcome of a guard check.
type Verdict int

const (
	Allow Verdict = iota
	Deny
	RateLimited
	Skip // Message should be skipped (e.g., no prefix in group)
)

// bucket tracks rate limit state for a single sender.
type bucket struct {
	tokens    int
	windowEnd time.Time
}

// Guard enforces sender allowlist, rate limiting, role resolution, session isolation, and group support.
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
	// Group support
	groupPrefix string
	groupIDs    map[string]bool // set of allowed group IDs
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

	groupIDs := make(map[string]bool)
	for _, id := range cfg.GroupIDs {
		groupIDs[strings.TrimSpace(id)] = true
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
		groupPrefix: cfg.GroupPrefix,
		groupIDs:    groupIDs,
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
	return baseKey + "-wa-" + n
}

// normalize strips all non-digit characters (including a leading +) so that
// "+15551234567" and "15551234567" both become "15551234567". This is required
// because the Meta/WhatsApp webhook sends `from` without a leading +, while
// config entries are commonly written with one.
func normalize(phone string) string {
	if phone == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(phone))

	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// IsGroup checks if a conversation ID represents a WhatsApp group.
// WhatsApp group IDs end with "@g.us".
func (g *Guard) IsGroup(conversationID string) bool {
	return strings.HasSuffix(conversationID, "@g.us")
}

// GroupPrefix returns the configured prefix for group messages.
func (g *Guard) GroupPrefix() string {
	return g.groupPrefix
}

// HasGroupPrefix checks if the text starts with the group prefix.
func (g *Guard) HasGroupPrefix(text string) bool {
	if g.groupPrefix == "" {
		return true // No prefix required
	}
	return strings.HasPrefix(strings.TrimSpace(text), g.groupPrefix)
}

// StripPrefix removes the group prefix from text if present.
func (g *Guard) StripPrefix(text string) string {
	if g.groupPrefix == "" {
		return text
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, g.groupPrefix) {
		return strings.TrimSpace(text[len(g.groupPrefix):])
	}
	return text
}

// IsGroupAllowed checks if a group ID is in the allowed list.
// If no groups are configured, all groups are allowed.
func (g *Guard) IsGroupAllowed(groupID string) bool {
	if len(g.groupIDs) == 0 {
		return true // No group filter configured
	}
	return g.groupIDs[groupID]
}

// CheckGroup verifies if a message from a group should be processed.
// Returns Allow if the group is allowed, user is authorized, and prefix matches.
// Returns Skip if prefix is required but not present (silent ignore).
// Returns Deny if group or user is not authorized.
func (g *Guard) CheckGroup(from, groupID, text string) Verdict {
	// Check if group is allowed
	if !g.IsGroupAllowed(groupID) {
		return Deny
	}

	// Check if user is authorized
	n := normalize(from)
	if g.mode == "allowlist" {
		if _, ok := g.phoneTo[n]; !ok {
			return Deny
		}
	}

	// Check prefix
	if !g.HasGroupPrefix(text) {
		return Skip
	}

	// Rate limiting
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
