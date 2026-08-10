// In-memory per-IP request rate limiting.
package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/response"
)

// bucket holds the token state for a single client IP.
type bucket struct {
	tokens float64
	last   time.Time
}

// rateLimiter is a token-bucket limiter keyed by client IP. It is safe for
// concurrent use. Entries idle longer than bucketTTL are evicted lazily so
// the map does not grow without bound.
type rateLimiter struct {
	mu      sync.Mutex
	rate    float64 // tokens refilled per second
	burst   float64 // maximum tokens a bucket can hold
	buckets map[string]*bucket
}

const (
	bucketTTL = 1 * time.Minute
	// sweepInterval is how often to check for idle buckets.
	sweepInterval = bucketTTL
)

// newRateLimiter builds a limiter that allows `rpm` requests per minute per
// IP with a burst of the same size (a full minute of headroom).
func newRateLimiter(rpm int) *rateLimiter {
	if rpm < 1 {
		rpm = 1
	}
	return &rateLimiter{
		rate:    float64(rpm) / float64(time.Minute.Seconds()),
		burst:   float64(rpm),
		buckets: make(map[string]*bucket),
	}
}

// allow reports whether a request from ip may proceed, consuming one token.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{tokens: rl.burst, last: now}
		rl.buckets[ip] = b
	}

	// Refill at rate tokens/second, capped at burst.
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = min(rl.burst, b.tokens+elapsed*rl.rate)
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep evicts buckets idle longer than bucketTTL. Called after a mutate so
// the lock is uncontended in the common path.
func (rl *rateLimiter) sweep() {
	now := time.Now()
	for ip, b := range rl.buckets {
		if now.Sub(b.last) > bucketTTL {
			delete(rl.buckets, ip)
		}
	}
}

// RateLimit returns Gin middleware enforcing a per-IP budget of rpm requests
// per minute. When the budget is exhausted it responds 429 with a
// Retry-After header and aborts the request.
func RateLimit(rpm int) gin.HandlerFunc {
	rl := newRateLimiter(rpm)
	var lastSweep time.Time

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.Header("Retry-After", "60")
			response.Error(c, sharederr.ErrRateLimited)
			c.Abort()
			return
		}

		// Lazy, rate-limited sweep to keep the map bounded.
		if time.Since(lastSweep) > sweepInterval {
			rl.sweep()
			lastSweep = time.Now()
		}
		c.Next()
	}
}
