// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"
)

var _ Observer = (*SequenceObserver)(nil)

// SequenceObserver provides ability to run observers from the list sequentially through next loop iterations.
// TODO find better name.
type SequenceObserver struct {
	observers       []Observer
	currentObserver int
}

// NewSequenceObserver creates new SequenceObserver instance.
func NewSequenceObserver(observers ...Observer) *SequenceObserver {
	return &SequenceObserver{
		observers: observers,
	}
}

// Start passes Start operation to current observer.
func (o *SequenceObserver) Start(ctx context.Context, startTime time.Time) (err error) {
	observer := o.observers[o.currentObserver]
	return observer.Start(ctx, startTime)
}

// Fork passes Fork operation to current observer.
func (o *SequenceObserver) Fork(ctx context.Context) (Partial, error) {
	observer := o.observers[o.currentObserver]
	return observer.Fork(ctx)
}

// Join passes Join operation to current observer.
func (o *SequenceObserver) Join(ctx context.Context, partial Partial) error {
	observer := o.observers[o.currentObserver]
	return observer.Join(ctx, partial)
}

// Finish passes Finish operation to current observer.
func (o *SequenceObserver) Finish(ctx context.Context) (err error) {
	observer := o.observers[o.currentObserver]
	if err := observer.Finish(ctx); err != nil {
		return err
	}

	o.currentObserver = (o.currentObserver + 1) % len(o.observers)
	return nil
}
