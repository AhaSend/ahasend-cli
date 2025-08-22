package client

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter wraps the golang rate limiter for API requests
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a new rate limiter
// rps: requests per second
// burst: burst capacity
func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// Wait blocks until the rate limiter permits another request
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

// Allow reports whether an event may happen now
func (rl *RateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

// Reserve returns a Reservation that indicates how long the caller
// must wait before the action is permitted
func (rl *RateLimiter) Reserve() *rate.Reservation {
	return rl.limiter.Reserve()
}

// GetWaitTime returns how long the caller must wait before the next action is permitted
func (rl *RateLimiter) GetWaitTime() time.Duration {
	reservation := rl.limiter.Reserve()
	defer reservation.Cancel()
	return reservation.Delay()
}
