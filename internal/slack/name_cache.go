package slack

import (
	"context"
	"sync"
	"time"
)

// nameLookup is the underlying resolver CachedNameResolver wraps.
type nameLookup interface {
	DisplayName(ctx context.Context, teamID, userID string) (string, error)
}

type cachedName struct {
	name      string
	expiresAt time.Time
}

// CachedNameResolver memoizes display-name lookups per (team, user) with a TTL.
// It avoids hammering users.info on group ++ and repeat awards. Failures are not
// cached, so a transient error retries (and the caller degrades to a mention).
type CachedNameResolver struct {
	inner nameLookup
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	entries map[string]cachedName
}

func NewCachedNameResolver(inner nameLookup, ttl time.Duration) *CachedNameResolver {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &CachedNameResolver{
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cachedName),
	}
}

func (c *CachedNameResolver) DisplayName(ctx context.Context, teamID, userID string) (string, error) {
	key := teamID + "|" + userID
	now := c.now()

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now.Before(e.expiresAt) {
		c.mu.Unlock()
		return e.name, nil
	}
	c.mu.Unlock()

	name, err := c.inner.DisplayName(ctx, teamID, userID)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.entries[key] = cachedName{name: name, expiresAt: now.Add(c.ttl)}
	c.mu.Unlock()

	return name, nil
}
