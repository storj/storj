// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package webhook_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/satellite/webhook"
)

// defaultConfig returns a config tuned for fast tests.
func defaultConfig() webhook.Config {
	return webhook.Config{
		HTTPTimeout: 2 * time.Second,
		Concurrency: 4,
		MaxRetries:  2,
		RetryDelay:  10 * time.Millisecond,
	}
}

// startMockServer starts an HTTP server that calls respond for each request
// to decide the status code. The returned count reports how many requests
// have been received.
func startMockServer(t *testing.T, respond func(requestNum int) int) (url string, count func() int) {
	var n atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		num := n.Add(1)
		w.WriteHeader(respond(int(num)))
	}))
	t.Cleanup(server.Close)

	return server.URL, func() int { return int(n.Load()) }
}

// newService creates a service with the given config and schedules Close
// at the end of the test.
func newService(t *testing.T, cfg webhook.Config) *webhook.Service {
	s := webhook.New(zaptest.NewLogger(t), cfg)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	return s
}

func TestService(t *testing.T) {
	t.Run("SendAsync succeeds", func(t *testing.T) {
		url, count := startMockServer(t, func(int) int { return http.StatusOK })

		s := newService(t, defaultConfig())
		s.SendAsync(t.Context(), url, []byte(`{"ok":true}`))
		s.TestWait()

		require.Equal(t, 1, count())
	})

	t.Run("SendAsync retries on failure", func(t *testing.T) {
		// Fail the first two attempts, succeed on the third.
		url, count := startMockServer(t, func(n int) int {
			if n <= 2 {
				return http.StatusBadRequest
			}
			return http.StatusOK
		})

		s := newService(t, defaultConfig())
		s.SendAsync(t.Context(), url, []byte(`{}`))
		s.TestWait()

		require.Equal(t, 3, count())
	})

	t.Run("SendAsync stops after MaxRetries", func(t *testing.T) {
		url, count := startMockServer(t, func(int) int { return http.StatusInternalServerError })

		cfg := defaultConfig()
		cfg.MaxRetries = 2
		s := newService(t, cfg)

		s.SendAsync(t.Context(), url, []byte(`{}`))
		s.TestWait()

		// cfg.MaxRetries+1 initial try
		require.Equal(t, cfg.MaxRetries+1, count())
	})

	t.Run("caller ctx does not cancel SendAsync", func(t *testing.T) {
		url, count := startMockServer(t, func(int) int { return http.StatusOK })

		s := newService(t, defaultConfig())

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		s.SendAsync(ctx, url, []byte(`{}`))
		s.TestWait()

		require.Equal(t, 1, count())
	})

	t.Run("Close aborts pending retries", func(t *testing.T) {
		url, count := startMockServer(t, func(int) int { return http.StatusInternalServerError })

		cfg := defaultConfig()
		cfg.MaxRetries = 10
		cfg.RetryDelay = time.Hour // Close must interrupt this delay
		s := webhook.New(zaptest.NewLogger(t), cfg)

		s.SendAsync(t.Context(), url, []byte(`{}`))

		// Wait for the first attempt so the sending is waiting to retry
		require.Eventually(t, func() bool { return count() >= 1 },
			5*time.Second, 10*time.Millisecond)

		require.NoError(t, s.Close())
		require.Equal(t, 1, count(), "no further attempts should fire after Close")
	})
}

func TestRetryDelay(t *testing.T) {
	newSvc := func(t *testing.T, base, maxDelay time.Duration) *webhook.Service {
		cfg := webhook.Config{
			HTTPTimeout:   time.Second,
			Concurrency:   1,
			RetryDelay:    base,
			MaxRetryDelay: maxDelay,
		}
		s := webhook.New(zaptest.NewLogger(t), cfg)
		t.Cleanup(func() { require.NoError(t, s.Close()) })
		return s
	}

	s := newSvc(t, 100*time.Millisecond, time.Second)
	cases := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"negative attempt uses to base", -3, 100 * time.Millisecond},
		{"zero attempt uses to base", 0, 100 * time.Millisecond},
		{"first retry uses base", 1, 100 * time.Millisecond},
		{"second retry doubles", 2, 200 * time.Millisecond},
		{"third retry quadruples", 3, 400 * time.Millisecond},
		{"fourth retry octuples", 4, 800 * time.Millisecond},
		{"fifth retry capped at max", 5, time.Second},
		{"large attempt capped at max", 10, time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, s.RetryDelay(c.attempt))
		})
	}
}
