// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Cycle implements a controllable recurring event.
//
// Cycle control methods don't have any effect after the cycle has completed.
type Cycle struct {
	interval time.Duration

	ticker  *time.Ticker
	control chan interface{}
	stop    chan struct{}

	init sync.Once
}

type (
	// cycle control messages
	cyclePause          struct{}
	cycleContinue       struct{}
	cycleStop           struct{}
	cycleChangeInterval struct{ Interval time.Duration }
	cycleTrigger        struct{ done chan struct{} }
)

// NewCycle creates a new cycle with the specified interval.
func NewCycle(interval time.Duration) *Cycle {
	cycle := &Cycle{}
	cycle.SetInterval(interval)
	return cycle
}

// SetInterval allows to change the interval before starting.
func (cycle *Cycle) SetInterval(interval time.Duration) {
	cycle.interval = interval
}

func (cycle *Cycle) initialize() {
	cycle.init.Do(func() {
		cycle.stop = make(chan struct{})
		cycle.control = make(chan interface{})
	})
}

// Start runs the specified function with an errgroup
func (cycle *Cycle) Start(ctx context.Context, group *errgroup.Group, fn func(ctx context.Context) error) {
	group.Go(func() error {
		return cycle.Run(ctx, fn)
	})
}

// Run runs the specified in an interval.
//
// Every interval `fn` is started.
// When `fn` is not fast enough, it may skip some of those executions.
func (cycle *Cycle) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	cycle.initialize()
	defer close(cycle.stop)

	currentInterval := cycle.interval
	cycle.ticker = time.NewTicker(currentInterval)
	if err := fn(ctx); err != nil {
		return err
	}
	for {
		select {

		case message := <-cycle.control:
			// handle control messages

			switch message := message.(type) {
			case cycleStop:
				return nil

			case cycleChangeInterval:
				currentInterval = message.Interval
				cycle.ticker.Stop()
				cycle.ticker = time.NewTicker(currentInterval)

			case cyclePause:
				cycle.ticker.Stop()
				// ensure we don't have ticks left
				select {
				case <-cycle.ticker.C:
				default:
				}

			case cycleContinue:
				cycle.ticker.Stop()
				cycle.ticker = time.NewTicker(currentInterval)

			case cycleTrigger:
				// trigger the function
				if err := fn(ctx); err != nil {
					return err
				}
				if message.done != nil {
					message.done <- struct{}{}
				}
			}

		case <-ctx.Done():
			// handle control messages
			return ctx.Err()

		case <-cycle.ticker.C:
			// trigger the function
			if err := fn(ctx); err != nil {
				return err
			}
		}
	}
}

// Close closes all resources associated with it.
func (cycle *Cycle) Close() {
	cycle.Stop()
	<-cycle.stop
	close(cycle.control)
}

// sendControl sends a control message
func (cycle *Cycle) sendControl(message interface{}) {
	cycle.initialize()
	select {
	case cycle.control <- message:
	case <-cycle.stop:
	}
}

// Stop stops the cycle permanently
func (cycle *Cycle) Stop() {
	cycle.sendControl(cycleStop{})
}

// ChangeInterval allows to change the ticker interval after it has started.
func (cycle *Cycle) ChangeInterval(interval time.Duration) {
	cycle.sendControl(cycleChangeInterval{interval})
}

// Pause pauses the cycle.
func (cycle *Cycle) Pause() {
	cycle.sendControl(cyclePause{})
}

// Restart restarts the ticker from 0.
func (cycle *Cycle) Restart() {
	cycle.sendControl(cycleContinue{})
}

// Trigger ensures that the loop is done at least once.
// If it's currently running it waits for the previous to complete and then runs.
func (cycle *Cycle) Trigger() {
	cycle.sendControl(cycleTrigger{})
}

// TriggerWait ensures that the loop is done at least once and waits for completion.
// If it's currently running it waits for the previous to complete and then runs.
func (cycle *Cycle) TriggerWait() {
	done := make(chan struct{})
	defer close(done)

	cycle.sendControl(cycleTrigger{done})
	select {
	case <-done:
	case <-cycle.stop:
	}
}
