// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package webhook provides a service for sending HTTP webhook notifications
// asynchronously with bounded concurrency.
package webhook

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/sync2"
)

var mon = monkit.Package()

// Error is the error class for the webhook package.
var Error = errs.Class("webhook")

// Config holds configuration for the webhook notification service.
type Config struct {
	HTTPTimeout   time.Duration `help:"timeout for webhook HTTP requests" default:"10s"`
	Concurrency   int           `help:"maximum concurrent webhook sends" default:"5"`
	MaxRetries    int           `help:"maximum additional attempts after initial webhook send failure" default:"3"`
	RetryDelay    time.Duration `help:"base delay between webhook retry attempts; doubles each retry up to MaxRetryDelay" default:"1s"`
	MaxRetryDelay time.Duration `help:"maximum delay between webhook retry attempts" default:"30s"`
}

// Service sends HTTP webhook notifications asynchronously with bounded concurrency.
//
// architecture: Service
type Service struct {
	log     *zap.Logger
	client  *http.Client
	config  Config
	limiter sync2.Limiter

	// wg is a wait group that primarily allows tests
	// to wait for inflight sends to finish.
	wg sync.WaitGroup

	stop     chan struct{}
	stopOnce sync.Once
}

// New creates a new Service.
func New(log *zap.Logger, config Config) *Service {
	return &Service{
		log:     log,
		client:  &http.Client{Timeout: config.HTTPTimeout},
		config:  config,
		limiter: *sync2.NewLimiter(config.Concurrency),
		stop:    make(chan struct{}),
	}
}

// SendAsync sends an HTTP POST of payload to url in the background,
// returning immediately. The sending is detached from the
// caller's context so it outlives the originating request, but it is still
// interruptible by Close.
func (s *Service) SendAsync(ctx context.Context, url string, payload []byte) {
	detached := context2.WithoutCancellation(ctx)

	s.wg.Add(1)
	started := s.limiter.Go(detached, func() {
		defer s.wg.Done()
		s.send(detached, url, payload)
	})
	if !started {
		s.wg.Done()
		s.log.Warn("webhook send skipped: service is closing", zap.String("url", url))
	}
}

func (s *Service) send(ctx context.Context, url string, payload []byte) {
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(s.RetryDelay(attempt)):
			case <-s.stop:
				return
			}
		}

		err := s.trySend(ctx, url, payload)
		if err == nil {
			return
		}
		s.log.Warn("webhook send failed, will retry",
			zap.String("url", url),
			zap.Int("attempt", attempt+1),
			zap.Int("max_attempts", s.config.MaxRetries),
			zap.Error(err))
	}
	s.log.Error("webhook send failed after all retries", zap.String("url", url))
}

// RetryDelay returns the backoff delay before retry attempt.
// It doubles RetryDelay each attempt, capped at MaxRetryDelay. If
// MaxRetryDelay is not greater than RetryDelay, the delay is constant.
func (s *Service) RetryDelay(attempt int) time.Duration {
	maxDelay := max(s.config.MaxRetryDelay, s.config.RetryDelay)
	exp := float64(max(attempt-1, 0))
	delay := time.Duration(float64(s.config.RetryDelay) * math.Pow(2, exp))

	if delay <= 0 || delay > maxDelay {
		return maxDelay
	}
	return delay
}

func (s *Service) trySend(ctx context.Context, url string, payload []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
			s.log.Debug("failed to drain webhook response body", zap.Error(drainErr))
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.log.Debug("failed to close webhook response body", zap.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return Error.New("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Close signals running sends to abort retries and waits for them to finish.
func (s *Service) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.limiter.Wait()
	s.wg.Wait()
	return nil
}

// TestWait blocks until all in-flight sends complete. Intended for tests only.
func (s *Service) TestWait() {
	s.wg.Wait()
}
