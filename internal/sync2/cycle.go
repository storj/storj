// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
	"time"
)

// Cycle implements a controllable recurring event.
type Cycle struct {
	interval time.Duration

	ticker  *time.Ticker
	control chan interface{}
	quit    chan struct{}

	init sync.Once
}

type (
	// cycle control messages
	cyclePause    struct{}
	cycleContinue struct{}
	cycleTrigger  struct {
		done chan struct{}
	}
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

// sendControl sends a control message
func (cycle *Cycle) sendControl(message interface{}) {
	select {
	case cycle.control <- message:
	case <-cycle.quit:
	}
}

// Run runs the specified function.
func (cycle *Cycle) Run(ctx context.Context, fn func() error) error {
	cycle.quit = make(chan struct{})
	defer close(cycle.quit)

	currentInterval := cycle.interval

	cycle.ticker = time.NewTicker(currentInterval)
	cycle.control = make(chan interface{})

	if err := fn(); err != nil {
		return err
	}
	for {
		select {
		case <-cycle.ticker.C:
			// trigger the function
			if err := fn(); err != nil {
				return err
			}

		case message := <-cycle.control:
			// handle control messages

			switch message := message.(type) {
			case nil:
				return nil

			case time.Duration:
				currentInterval = message
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
				if err := fn(); err != nil {
					return err
				}
				if message.done != nil {
					message.done <- struct{}{}
				}
			}

		case <-ctx.Done():
			// handle control messages
			return ctx.Err()
		}
	}
}

// Stop stops the cycle permanently
func (cycle *Cycle) Stop() {
	cycle.sendControl(nil)
}

// ChangeInterval allows to change the ticker interval after it has started.
func (cycle *Cycle) ChangeInterval(interval time.Duration) {
	cycle.sendControl(interval)
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
	cycle.sendControl(cycleTrigger{done})
	select {
	case <-done:
	case <-cycle.quit:
	}
}
