// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion

import (
	"context"

	"golang.org/x/sync/semaphore"

	"storj.io/common/storj"
)

// LimitedHandler wraps handler with a concurrency limit.
type LimitedHandler struct {
	active *semaphore.Weighted
	Handler
}

// NewLimitedHandler wraps handler with a concurrency limit.
func NewLimitedHandler(handler Handler, limit int) *LimitedHandler {
	return &LimitedHandler{
		active:  semaphore.NewWeighted(int64(limit)),
		Handler: handler,
	}
}

// Handle handles the job queue.
func (handler *LimitedHandler) Handle(ctx context.Context, node storj.NodeURL, queue Queue) {
	if err := handler.active.Acquire(ctx, 1); err != nil {
		return
	}
	defer handler.active.Release(1)

	handler.Handler.Handle(ctx, node, queue)
}
