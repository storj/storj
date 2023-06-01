// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedlooptest

import (
	"context"
	"runtime"
	"time"

	"storj.io/storj/satellite/metabase/rangedloop"
)

var _ rangedloop.Observer = (*CallbackObserver)(nil)
var _ rangedloop.Partial = (*CallbackObserver)(nil)

// CallbackObserver can be used to easily attach logic to the ranged segment loop during tests.
type CallbackObserver struct {
	OnProcess func(context.Context, []rangedloop.Segment) error
	OnStart   func(context.Context, time.Time) error
	OnFork    func(context.Context) (rangedloop.Partial, error)
	OnJoin    func(context.Context, rangedloop.Partial) error
	OnFinish  func(context.Context) error
}

// delay ensures that using time.Now can be used to measure a visible duration.
func delay() {
	if runtime.GOOS == "windows" {
		// Windows time measurement is especially bad, so, we need to sleep more than
		// for other environments.
		time.Sleep(time.Millisecond)
	} else {
		time.Sleep(time.Microsecond)
	}
}

// Start executes a callback at ranged segment loop start.
func (c *CallbackObserver) Start(ctx context.Context, time time.Time) error {
	delay()
	if c.OnStart == nil {
		return nil
	}

	return c.OnStart(ctx, time)
}

// Fork executes a callback for every segment range at ranged segment loop fork stage.
func (c *CallbackObserver) Fork(ctx context.Context) (rangedloop.Partial, error) {
	delay()
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
	delay()
	if c.OnJoin == nil {
		return nil
	}

	return c.OnJoin(ctx, partial)
}

// Finish executes a callback at ranged segment loop end.
func (c *CallbackObserver) Finish(ctx context.Context) error {
	delay()
	if c.OnFinish == nil {
		return nil
	}

	return c.OnFinish(ctx)
}

// Process executes a callback for every batch of segment in the ranged segment loop.
func (c *CallbackObserver) Process(ctx context.Context, segments []rangedloop.Segment) error {
	delay()
	if c.OnProcess == nil {
		return nil
	}

	return c.OnProcess(ctx, segments)
}
