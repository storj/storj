// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package groupcancel

import (
	"context"
	"sync"
	"time"
)

// _context embeds a context.Context so that it can be embed without exporting
// a field named Context for external packages to inspect or modify.
type _context struct{ context.Context }

// Context keeps track of a set of operations and helps cancel long tails.
type Context struct {
	_context
	cancel func()

	start     time.Time
	cancelAt  float64
	extraWait float64

	mu       sync.Mutex
	canceled bool
	total    int
	good     int
	bad      int
	timer    *time.Timer
}

// NewContext constructs a Context which implements context.Context and allows one to
// cancel it based on the speed at which some operations completed.
//
// When the number of successful operations vs the number of non-bad remaining operations
// exceeds the ratio to cancel at, the Context will cancel after waiting an amount of time
// computed by the amount of time it took so far multiplied by extraWait.
//
// It returns the Context and a cancel func that must be called, much like the context api.
func NewContext(ctx context.Context, total int, cancelAt float64, extraWait float64) (*Context, func()) {
	ctx, cancel := context.WithCancel(ctx)

	c := &Context{
		_context: _context{Context: ctx},
		cancel:   cancel,

		start:     time.Now(),
		cancelAt:  cancelAt,
		extraWait: extraWait,

		canceled: false,
		total:    total,
		good:     0,
		bad:      0,
	}

	return c, c.lockedCancel
}

// ensure that our context type implements context.Context as expected.
var _ context.Context = (*Context)(nil)

// lockedCancel acquires the mutex and issues a cancel.
func (c *Context) lockedCancel() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.issueCancel()
}

// issueCancel does the work to cancel. It must be called with the mutex held.
func (c *Context) issueCancel() {
	c.cancel()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.canceled = true
}

// Success tells the Context that one of the operations was successful.
func (c *Context) Success() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.good++
	c.checkCancel()
}

// Failure tells the Context that one of the operations was a failure.
func (c *Context) Failure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.bad++
	c.checkCancel()
}

// checkCancel checks the state of the Context and either directly issues
// a cancel or sets a timer to do so.
func (c *Context) checkCancel() {
	switch {
	// if the timer is set, we're already done.
	case c.timer != nil:

	// if we're already canceled, we're already done.
	case c.canceled:

	// if we've reached/surpassed the total, we're done. issue the cancel.
	case c.good+c.bad >= c.total:
		c.issueCancel()

	// if our ratio exceeds cancelAt, set the timer to issue the cancel.
	case float64(c.good)/float64(c.total-c.bad) >= c.cancelAt:
		delay := time.Duration(float64(time.Since(c.start)) * c.extraWait)
		c.timer = time.AfterFunc(delay, c.lockedCancel)
	}
}
