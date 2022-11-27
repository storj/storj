// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var _ rangedloop.Observer = (*CallbackObserver)(nil)
var _ rangedloop.Partial = (*CallbackObserver)(nil)

// CallbackObserver can be used to easily attach logic to the ranged segment loop during tests.
type CallbackObserver struct {
	OnProcess func(context.Context, []segmentloop.Segment) error
	OnStart   func(context.Context, time.Time) error
	OnFork    func(context.Context) (rangedloop.Partial, error)
	OnJoin    func(context.Context, rangedloop.Partial) error
	OnFinish  func(context.Context) error
}

// Start executes a callback at ranged segment loop start.
func (c *CallbackObserver) Start(ctx context.Context, time time.Time) error {
	if c.OnStart == nil {
		return nil
	}

	return c.OnStart(ctx, time)
}

// Fork executes a callback for every segment range at ranged segment loop fork stage.
func (c *CallbackObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
	if c.OnFork == nil {
		return c, nil
	}

	partial, err := c.OnFork(ctx)
	if err != nil {
		return nil, err
	}

	if partial == nil {
		return c, nil
	}

	return partial, nil
}

// Join executes a callback for every segment range at ranged segment loop join stage.
func (c *CallbackObserver) Join(ctx context.Context, partial rangedloop.Partial) error {
	if c.OnJoin == nil {
		return nil
	}

	return c.OnJoin(ctx, partial)
}

// Finish executes a callback at ranged segment loop end.
func (c *CallbackObserver) Finish(ctx context.Context) error {
	if c.OnFinish == nil {
		return nil
	}

	return c.OnFinish(ctx)
}

// Process executes a callback for every batch of segment in the ranged segment loop.
func (c *CallbackObserver) Process(ctx context.Context, segments []segmentloop.Segment) error {
	if c.OnProcess == nil {
		return nil
	}

	return c.OnProcess(ctx, segments)
}
