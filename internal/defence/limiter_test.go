package defence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter(t *testing.T) {
	key1 := "email1@example.com"
	key2 := "email2@example.com"
	maxAttempts := 3
	attemptsPeriod := time.Second
	banDuration := time.Second
	clearPeriod := time.Second * 3

	var limiter = NewLimiter(maxAttempts, attemptsPeriod, banDuration, clearPeriod)

	t.Run("Testing constructor", func(t *testing.T) {
		assert.Equal(t, limiter.Attempts, maxAttempts)
		assert.Equal(t, limiter.AttemptsPeriod, attemptsPeriod)
		assert.Equal(t, limiter.BanDuration, banDuration)
		assert.Equal(t, limiter.ClearPeriod, clearPeriod)
		assert.NotNil(t, limiter.Attempts)
	})

	t.Run("should not be banned when to attack < attempts during attemptsPeriod", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			limiter.Attack(key1)
		}

		assert.Equal(t, len(limiter.Banned()), 0)
		attacker, ok := limiter.Find(key1)
		assert.Equal(t, ok, true)
		assert.Equal(t, attacker.IsBanned, false)
	})

	t.Run("should be banned when attack > attempts when attemptsPeriod exceeded", func(t *testing.T) {
		for i := 0; i <= maxAttempts; i++ {
			limiter.Attack(key2)
		}

		assert.Equal(t, len(limiter.Banned()), 1)
		attacker, ok := limiter.Find(key2)
		assert.Equal(t, ok, true)
		assert.Equal(t, attacker.IsBanned, true)
	})

	t.Run("clear works fine", func(t *testing.T) {
		for i := 0; i <= maxAttempts; i++ {
			limiter.Attack(key1)
			limiter.Attack(key2)
		}

		assert.Equal(t, len(limiter.Banned()), 2)

		limiter.CleanUp()

		ticker := time.NewTicker(clearPeriod)

		for range ticker.C {
			assert.Equal(t, len(limiter.Banned()), 0)
			ticker.Stop()
		}
	})
}
