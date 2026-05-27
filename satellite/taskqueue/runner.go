// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package taskqueue

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"storj.io/common/sync2"
)

// RunnerConfig contains configuration for the task queue runner.
type RunnerConfig struct {
	WorkerCount int           `help:"number of concurrent workers" default:"10"`
	Interval    time.Duration `help:"how often to check for new jobs" default:"15s"`
	BatchSize   int           `help:"number of jobs to pop from queue at once" default:"100"`
}

// Processor defines the interface for processing jobs from the task queue.
// Implementations handle the actual work for each job.
type Processor[T any] interface {
	// Process handles a single job. It is called concurrently from multiple workers.
	Process(ctx context.Context, job T)
}

// ProcessorFunc is a function adapter for Processor.
type ProcessorFunc[T any] func(ctx context.Context, job T)

// Process implements Processor.
func (f ProcessorFunc[T]) Process(ctx context.Context, job T) {
	f(ctx, job)
}

// Runner pops jobs from a task queue stream and processes them concurrently.
type Runner[T any] struct {
	log    *zap.Logger
	config RunnerConfig

	client    *Client
	streamID  string
	processor Processor[T]

	JobLimiter *semaphore.Weighted
}

// NewRunner creates a new task queue runner.
func NewRunner[T any](
	log *zap.Logger,
	config RunnerConfig,
	client *Client,
	streamID string,
	processor Processor[T],
) *Runner[T] {
	return &Runner[T]{
		log:        log,
		config:     config,
		client:     client,
		streamID:   streamID,
		processor:  processor,
		JobLimiter: semaphore.NewWeighted(int64(config.WorkerCount)),
	}
}

// Run starts the runner loop.
func (r *Runner[T]) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err, empty := r.processJobs(ctx)
		if err != nil {
			r.log.Error("failed to process jobs", zap.Error(err))
		}
		if empty || err != nil {
			if !sync2.Sleep(ctx, r.config.Interval) {
				return ctx.Err()
			}
		}
	}
}

// Close stops the runner loop.
func (r *Runner[T]) Close() error {
	return nil
}

func (r *Runner[T]) processJobs(ctx context.Context) (err error, empty bool) {
	defer mon.Task()(&ctx)(&err)

	rawItems, err := r.client.PopBatch(ctx, r.streamID, int64(r.config.BatchSize), time.Second, func() any {
		return new(T)
	})
	if err != nil {
		return err, false
	}

	if len(rawItems) == 0 {
		return nil, true
	}

	jobs := make([]T, len(rawItems))
	for i, item := range rawItems {
		jobs[i] = *item.(*T)
	}

	r.log.Debug("processing jobs", zap.Int("count", len(jobs)))

	var wg sync.WaitGroup
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			break
		}

		if err := r.JobLimiter.Acquire(ctx, 1); err != nil {
			break
		}

		wg.Add(1)
		job := job
		go func() {
			defer wg.Done()
			defer r.JobLimiter.Release(1)
			r.processor.Process(ctx, job)
		}()
	}

	wg.Wait()
	return nil, false
}
