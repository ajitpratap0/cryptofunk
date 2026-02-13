package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(5) // 5 per minute for testing

	userID := int64(123)

	// First 5 should pass
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow(userID), "request %d should be allowed", i+1)
	}

	// 6th should be blocked
	assert.False(t, rl.Allow(userID), "6th request should be blocked")

	// Different user should still be allowed
	assert.True(t, rl.Allow(int64(456)), "different user should be allowed")
}

func TestRateLimiter_DifferentUsers(t *testing.T) {
	rl := NewRateLimiter(2)

	assert.True(t, rl.Allow(1))
	assert.True(t, rl.Allow(2))
	assert.True(t, rl.Allow(1))
	assert.False(t, rl.Allow(1)) // user 1 exhausted
	assert.True(t, rl.Allow(2))  // user 2 still has 1
	assert.False(t, rl.Allow(2)) // user 2 exhausted
}
