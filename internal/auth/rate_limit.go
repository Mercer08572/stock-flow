package auth

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type RateLimitPolicy struct {
	MaxAttempts int
	Window      time.Duration
	Lockout     time.Duration
}

type LoginRateLimiter interface {
	Check(ctx context.Context, key string, policy RateLimitPolicy) (time.Duration, error)
	Record(ctx context.Context, key string, policy RateLimitPolicy) error
	Reset(ctx context.Context, key string) error
}

type rateLimitEntry struct {
	attempts    []time.Time
	lockedUntil time.Time
}

type inMemoryLoginRateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateLimitEntry
	now     func() time.Time
}

func NewInMemoryLoginRateLimiter(now func() time.Time) LoginRateLimiter {
	if now == nil {
		now = time.Now
	}
	return &inMemoryLoginRateLimiter{entries: make(map[string]rateLimitEntry), now: now}
}

func (l *inMemoryLoginRateLimiter) Check(_ context.Context, key string, policy RateLimitPolicy) (time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry := l.prune(key, policy, now)
	if now.Before(entry.lockedUntil) {
		return entry.lockedUntil.Sub(now), nil
	}
	return 0, nil
}

func (l *inMemoryLoginRateLimiter) Record(_ context.Context, key string, policy RateLimitPolicy) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry := l.prune(key, policy, now)
	entry.attempts = append(entry.attempts, now)
	if len(entry.attempts) >= policy.MaxAttempts {
		entry.lockedUntil = now.Add(policy.Lockout)
	}
	l.entries[key] = entry
	return nil
}

func (l *inMemoryLoginRateLimiter) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
	return nil
}

func (l *inMemoryLoginRateLimiter) prune(key string, policy RateLimitPolicy, now time.Time) rateLimitEntry {
	entry := l.entries[key]
	cutoff := now.Add(-policy.Window)
	kept := entry.attempts[:0]
	for _, attempt := range entry.attempts {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}
	entry.attempts = kept
	if !entry.lockedUntil.IsZero() && !now.Before(entry.lockedUntil) {
		entry.lockedUntil = time.Time{}
		entry.attempts = nil
	}
	l.entries[key] = entry
	return entry
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

// Headers 让错误出口补上 Retry-After，客户端据此知道多久后可以重试。
func (e *RateLimitError) Headers() map[string]string {
	seconds := int64(e.RetryAfter.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}

	return map[string]string{"Retry-After": strconv.FormatInt(seconds, 10)}
}
