package slack

import (
	"sync"
	"time"
)

// reactionReplyCooldown is how long a reactor stays unanswered after a reply for the
// same award on the same message.
const reactionReplyCooldown = 10 * time.Minute

// replyCooldown remembers recent reaction replies, so toggling a reaction on and off
// doesn't post a reply (or count as a new karma-war interaction) every time.
// ponytail: in memory, so a restart forgets it (one extra reply at most), and it assumes
// one replica; move it to a TTL'd DynamoDB row if the app ever runs more than one.
type replyCooldown struct {
	window     time.Duration
	now        func() time.Time
	mu         sync.Mutex
	until      map[string]time.Time
	maxEntries int
}

func newReplyCooldown(window time.Duration) *replyCooldown {
	return &replyCooldown{window: window, now: time.Now, until: map[string]time.Time{}, maxEntries: 10_000}
}

func (c *replyCooldown) active(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Before(c.until[key])
}

func (c *replyCooldown) start(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.until) >= c.maxEntries {
		for k, t := range c.until {
			if !now.Before(t) {
				delete(c.until, k)
			}
		}
		// ponytail: if every entry is still live, start over; worst case a few extra replies.
		if len(c.until) >= c.maxEntries {
			clear(c.until)
		}
	}
	c.until[key] = now.Add(c.window)
}
