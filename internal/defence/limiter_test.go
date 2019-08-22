// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package defence

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
)

const (
	key1         = "email1@example.com"
	key2         = "email2@example.com"
	maxAttempts  = 3
	lockInterval = time.Second
	clearPeriod  = time.Second * 3
)

func TestLimiter(t *testing.T) {
	var limiter = NewLimiter(maxAttempts, lockInterval, clearPeriod)
	defer limiter.Close()

	t.Run("Testing constructor", func(t *testing.T) {
		assert.Equal(t, limiter.attempts, maxAttempts)
		assert.Equal(t, limiter.lockInterval, lockInterval)
		assert.NotNil(t, limiter.attempts)
	})

	t.Run("should not be banned when to attack < attempts during attemptsPeriod", func(t *testing.T) {
		result := false

		for i := 0; i < 2; i++ {
			result = limiter.Limit(key1)
		}

		assert.Equal(t, result, true)
	})

	t.Run("should be banned when attack > attempts when attemptsPeriod exceeded", func(t *testing.T) {
		result := false

		for i := 0; i <= maxAttempts; i++ {
			result = limiter.Limit(key2)
		}

		assert.Equal(t, result, false)
	})
}

func TestLimiterConcurrent(t *testing.T) {
	textContext := testcontext.New(t)
	ctx, cancel := context.WithCancel(textContext)

	defer textContext.Cleanup()
	defer cancel()

	var wg sync.WaitGroup
	limiter := NewLimiter(maxAttempts, lockInterval, clearPeriod)

	defer limiter.Close()

	go func() {
		err := limiter.Run(ctx)
		assert.Error(t, err)
		return
	}()

	wg.Add(1)
	go processFirstAttacker(t, &wg, limiter)

	wg.Add(1)
	go processSecondAttacker(t, &wg, limiter)

	wg.Wait()
}

// first attacker performs 3 operation ( with 3 max attempts ) per ~4 seconds ( with attempt duration 1 sec)
// so it should not be locked.
func processFirstAttacker(t *testing.T, wg *sync.WaitGroup, limiter *Limiter) {
	defer wg.Done()

	ticker := time.NewTicker(time.Second)
	i := 0
	result := false

	for range ticker.C {
		if i == maxAttempts {
			ticker.Stop()
			break
		}

		result = limiter.Limit(key1)

		i++
	}

	assert.True(t, result)
}

// second attacker performs 4 operation ( with 3 max attempts ) per ~0.8 second ( with attempt duration 1 sec)
// so it should be locked.
func processSecondAttacker(t *testing.T, wg *sync.WaitGroup, limiter *Limiter) {
	defer wg.Done()

	ticker := time.NewTicker(time.Second / 5)
	i := 0
	result := false

	for range ticker.C {
		if i == 4 {
			ticker.Stop()
			break
		}

		result = limiter.Limit(key1)

		i++
	}

	assert.False(t, result)
}

// ExampleLimit shows how to use and close Limiter.
func ExampleLimit() {
	ctx := context.Background()
	limiter := NewLimiter(maxAttempts, lockInterval, clearPeriod)

	defer limiter.Close()

	go func() {
		if err := limiter.Run(ctx); err != nil {
			fmt.Print(err)
		}
	}()

	if !limiter.Limit("someKey") {
		return
	}

	// Output:
}

// ExampleLimit shows how to use and close (with context) Limiter
func ExampleLimit_second() {
	ctx, close := context.WithCancel(context.Background())
	limiter := NewLimiter(maxAttempts, lockInterval, clearPeriod)

	defer close()

	go func() {
		if err := limiter.Run(ctx); err != nil {
			fmt.Print(err)
		}
	}()

	if !limiter.Limit("someKey") {
		fmt.Print("you are banned")
	}

	// Output:
}
