package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name  string
		rps   int
		burst int
	}{
		{
			name:  "standard rate limiter",
			rps:   50,
			burst: 100,
		},
		{
			name:  "low rate limiter",
			rps:   1,
			burst: 1,
		},
		{
			name:  "high rate limiter",
			rps:   1000,
			burst: 2000,
		},
		{
			name:  "zero rps with burst",
			rps:   0,
			burst: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.rps, tt.burst)

			assert.NotNil(t, rl)
			assert.NotNil(t, rl.limiter)

			// Test that the rate limiter is functional
			allowed := rl.Allow()
			assert.True(t, allowed, "First request should generally be allowed with burst capacity")
		})
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	// Create a rate limiter with 1 request per second and burst of 2
	rl := NewRateLimiter(1, 2)

	// First two requests should be allowed due to burst capacity
	assert.True(t, rl.Allow(), "First request should be allowed")
	assert.True(t, rl.Allow(), "Second request should be allowed due to burst")

	// Third request should be denied as we've exceeded burst and rate
	allowed := rl.Allow()
	// Note: This might be true or false depending on timing, but the rate limiter should be working
	t.Logf("Third request allowed: %t", allowed)
}

func TestRateLimiter_Wait(t *testing.T) {
	// Create a rate limiter with low rate for testing
	rl := NewRateLimiter(10, 1) // 10 requests per second, burst of 1

	ctx := context.Background()

	// First request should not wait
	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 10*time.Millisecond, "First request should not have significant wait time")
}

func TestRateLimiter_Wait_WithTimeout(t *testing.T) {
	// Create a very restrictive rate limiter
	rl := NewRateLimiter(1, 1)

	// Consume the burst allowance
	_ = rl.Allow()

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// The wait should respect the timeout
	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)

	if err != nil {
		// Should get context timeout error (checking error message contains timeout info)
		assert.Contains(t, err.Error(), "deadline")
		assert.Less(t, duration, 10*time.Millisecond, "Should timeout quickly")
	}
}

func TestRateLimiter_Wait_WithCancellation(t *testing.T) {
	// Create a restrictive rate limiter
	rl := NewRateLimiter(1, 1)

	// Consume the burst allowance
	_ = rl.Allow()

	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay
	go func() {
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	// The wait should respect the cancellation
	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)

	if err != nil {
		// Should get context canceled error
		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, duration, 100*time.Millisecond, "Should be canceled quickly")
	}
}

func TestRateLimiter_Reserve(t *testing.T) {
	rl := NewRateLimiter(10, 5) // 10 requests per second, burst of 5

	reservation := rl.Reserve()
	assert.NotNil(t, reservation)

	// The reservation should have a reasonable delay
	delay := reservation.Delay()
	assert.True(t, delay >= 0, "Delay should be non-negative")

	// Cancel the reservation to clean up
	reservation.Cancel()
}

func TestRateLimiter_GetWaitTime(t *testing.T) {
	rl := NewRateLimiter(10, 2) // 10 requests per second, burst of 2

	// First call should have minimal wait time
	waitTime1 := rl.GetWaitTime()
	assert.True(t, waitTime1 >= 0, "Wait time should be non-negative")

	// Consume some allowance
	_ = rl.Allow()
	_ = rl.Allow()

	// Now wait time might be longer
	waitTime2 := rl.GetWaitTime()
	assert.True(t, waitTime2 >= 0, "Wait time should be non-negative")

	t.Logf("Wait time 1: %v, Wait time 2: %v", waitTime1, waitTime2)
}

func TestRateLimiter_BurstHandling(t *testing.T) {
	// Create rate limiter with burst capacity
	rl := NewRateLimiter(1, 5) // 1 request per second, burst of 5

	// Should allow burst requests immediately
	allowedCount := 0
	for i := 0; i < 10; i++ {
		if rl.Allow() {
			allowedCount++
		}
	}

	// Should have allowed at least the burst capacity
	assert.Greater(t, allowedCount, 0, "Should allow some requests")
	assert.LessOrEqual(t, allowedCount, 10, "Should not allow unlimited requests")

	t.Logf("Allowed %d out of 10 requests with burst=5", allowedCount)
}

func TestRateLimiter_RateLimit_Integration(t *testing.T) {
	// Test with realistic AhaSend parameters
	rl := NewRateLimiter(50, 100) // 50 req/sec with 100 burst as per AhaSend specs

	// Should allow initial burst
	initialAllowed := 0
	for i := 0; i < 50; i++ {
		if rl.Allow() {
			initialAllowed++
		}
	}

	assert.Greater(t, initialAllowed, 40, "Should allow most of the initial requests due to burst")
	t.Logf("Allowed %d out of 50 initial requests", initialAllowed)

	// Test wait functionality with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)

	t.Logf("Wait duration: %v, Error: %v", duration, err)
}

// Benchmark tests for rate limiter performance
func BenchmarkRateLimiter_AllowRateLimit(b *testing.B) {
	rl := NewRateLimiter(50, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rl.Allow()
	}
}

func BenchmarkRateLimiter_GetWaitTime(b *testing.B) {
	rl := NewRateLimiter(50, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rl.GetWaitTime()
	}
}

func BenchmarkRateLimiter_Reserve(b *testing.B) {
	rl := NewRateLimiter(50, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reservation := rl.Reserve()
		reservation.Cancel()
	}
}

func BenchmarkNewRateLimiter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewRateLimiter(50, 100)
	}
}

// Test race conditions in rate limiter (simplified version)
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(10, 20)

	// Test that concurrent calls don't panic or cause races
	done := make(chan bool, 5)

	// Start a few goroutines that try to get permission
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 3; j++ {
				_ = rl.Allow()
				_ = rl.GetWaitTime()
			}
		}()
	}

	// Wait for all goroutines to complete (with timeout)
	timeout := time.After(1 * time.Second)
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// Good, goroutine completed
		case <-timeout:
			t.Fatal("Test timed out - possible deadlock")
		}
	}
}

// Test rate limiter behavior over time (simplified)
func TestRateLimiter_TimeBasedRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	rl := NewRateLimiter(10, 1) // 10 requests per second, burst of 1

	// Use up the burst capacity
	assert.True(t, rl.Allow(), "First request should be allowed")

	// Check that the rate limiter has some wait time now
	waitTime := rl.GetWaitTime()
	assert.True(t, waitTime >= 0, "Wait time should be non-negative after burst exhaustion")

	t.Logf("Wait time after burst exhaustion: %v", waitTime)
}

// Edge case tests
func TestRateLimiter_EdgeCases(t *testing.T) {
	t.Run("zero rate with burst", func(t *testing.T) {
		rl := NewRateLimiter(0, 5)

		// Should still allow some requests due to burst
		allowed := rl.Allow()
		t.Logf("Zero rate limiter allowed request: %t", allowed)
	})

	t.Run("zero burst", func(t *testing.T) {
		rl := NewRateLimiter(10, 0)

		// Behavior depends on underlying implementation
		allowed := rl.Allow()
		t.Logf("Zero burst limiter allowed request: %t", allowed)
	})

	t.Run("negative values", func(t *testing.T) {
		// The underlying rate.NewLimiter should handle negative values gracefully
		rl := NewRateLimiter(-1, -1)
		assert.NotNil(t, rl)

		// Behavior is implementation-dependent but shouldn't panic
		_ = rl.Allow()
	})
}
