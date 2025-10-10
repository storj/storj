// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"

	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

// GcBfOnce is a subcommand to start only a ranged loop with BF generation.
type GcBfOnce struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *GcBfOnce) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	mud.RemoveTag[*bloomfilter.SyncObserverV2, mud.Optional](ball)
	mud.RemoveTag[*rangedloop.SegmentsCountValidation, mud.Optional](ball)

	mud.Provide[*OnceRunner](ball, NewOnceRunner)
	return mud.Or(
		mud.Select[debug.Wrapper](ball),
		mud.Select[*OnceRunner](ball))
}

// OnceRunner is a wrapper to run the ranged loop once and then stop the application.
type OnceRunner struct {
	service *rangedloop.Service
	trigger *modular.StopTrigger
}

// NewOnceRunner creates a new OnceRunner.
func NewOnceRunner(svc *rangedloop.Service, trigger *modular.StopTrigger) *OnceRunner {
	return &OnceRunner{
		service: svc,
		trigger: trigger,
	}
}

// Run runs the ranged loop once and then stops the application.
func (o *OnceRunner) Run(ctx context.Context) error {
	_, err := o.service.RunOnce(ctx)
	o.trigger.Cancel()
	return err
}

// Close closes the OnceRunner.
func (o *OnceRunner) Close() error {
	return o.service.Close()
}
