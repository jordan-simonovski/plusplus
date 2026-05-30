package slack

import (
	"context"
	"errors"
	"testing"
	"time"
)

type countingLookup struct {
	calls int
	name  string
	err   error
}

func (c *countingLookup) DisplayName(_ context.Context, _, _ string) (string, error) {
	c.calls++
	if c.err != nil {
		return "", c.err
	}
	return c.name, nil
}

func TestCachedNameResolverServesFromCache(t *testing.T) {
	inner := &countingLookup{name: "Jane"}
	r := NewCachedNameResolver(inner, time.Hour)

	for i := 0; i < 3; i++ {
		name, err := r.DisplayName(context.Background(), "T1", "U1")
		if err != nil || name != "Jane" {
			t.Fatalf("got %q, %v", name, err)
		}
	}
	if inner.calls != 1 {
		t.Fatalf("expected 1 upstream call, got %d", inner.calls)
	}
}

func TestCachedNameResolverKeysByTeamAndUser(t *testing.T) {
	inner := &countingLookup{name: "Jane"}
	r := NewCachedNameResolver(inner, time.Hour)

	_, _ = r.DisplayName(context.Background(), "T1", "U1")
	_, _ = r.DisplayName(context.Background(), "T2", "U1")
	_, _ = r.DisplayName(context.Background(), "T1", "U2")

	if inner.calls != 3 {
		t.Fatalf("expected distinct keys to miss, got %d calls", inner.calls)
	}
}

func TestCachedNameResolverRefetchesAfterTTL(t *testing.T) {
	inner := &countingLookup{name: "Jane"}
	r := NewCachedNameResolver(inner, time.Minute)
	now := time.Unix(0, 0)
	r.now = func() time.Time { return now }

	_, _ = r.DisplayName(context.Background(), "T1", "U1")
	now = now.Add(2 * time.Minute)
	_, _ = r.DisplayName(context.Background(), "T1", "U1")

	if inner.calls != 2 {
		t.Fatalf("expected refetch after TTL, got %d calls", inner.calls)
	}
}

func TestCachedNameResolverDoesNotCacheErrors(t *testing.T) {
	inner := &countingLookup{err: errors.New("boom")}
	r := NewCachedNameResolver(inner, time.Hour)

	if _, err := r.DisplayName(context.Background(), "T1", "U1"); err == nil {
		t.Fatalf("expected error")
	}
	inner.err = nil
	inner.name = "Jane"

	name, err := r.DisplayName(context.Background(), "T1", "U1")
	if err != nil || name != "Jane" {
		t.Fatalf("expected retry to succeed, got %q, %v", name, err)
	}
	if inner.calls != 2 {
		t.Fatalf("expected 2 calls (error not cached), got %d", inner.calls)
	}
}
