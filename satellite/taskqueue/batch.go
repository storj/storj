// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package taskqueue

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// BatchProcessor defines the interface for processing jobs from the task queue in batches.
// Implementations handle the actual work for a batch of jobs at once.
type BatchProcessor[T any] interface {
	// ProcessBatch handles a batch of jobs.
	ProcessBatch(ctx context.Context, jobs []T)
}

// BatchProcessorFunc is a function adapter for BatchProcessor.
type BatchProcessorFunc[T any] func(ctx context.Context, jobs []T)

// ProcessBatch implements BatchProcessor.
func (f BatchProcessorFunc[T]) ProcessBatch(ctx context.Context, jobs []T) {
	f(ctx, jobs)
}

// BatchRunner pops jobs from a task queue stream and processes them in batches.
type BatchRunner[T any] struct {
	log    *zap.Logger
	config RunnerConfig

	client    *Client
	streamID  string
	processor BatchProcessor[T]
}

// NewBatchRunner creates a new task queue batch runner.
func NewBatchRunner[T any](
	log *zap.Logger,
	config RunnerConfig,
	client *Client,
	streamID string,
	processor BatchProcessor[T],
) *BatchRunner[T] {
	return &BatchRunner[T]{
		log:       log,
		config:    config,
		client:    client,
		streamID:  streamID,
		processor: processor,
	}
}

// Run starts the batch runner loop.
func (r *BatchRunner[T]) Run(ctx context.Context) (err error) {
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

// Close stops the batch runner loop.
func (r *BatchRunner[T]) Close() error {
	return nil
}

func (r *BatchRunner[T]) processJobs(ctx context.Context) (err error, empty bool) {
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

	r.processor.ProcessBatch(ctx, jobs)

	return nil, false
}
