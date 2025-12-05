// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/shared/mud"
)

// StopTrigger is a helper to stop all the components and finish the process. Just call cancel.
type StopTrigger struct {
	Cancel context.CancelFunc
}

// Run runs storage node until it's either closed or it errors.
func Run(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) (err error) {
	eg := &errgroup.Group{}
	err = mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		return component.Run(ctx, eg)
	}, mud.All)
	if err != nil {
		return err
	}
	return eg.Wait()
}

// Close closes all the resources.
func Close(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = mud.ForEachDependencyReverse(ball, selector, func(component *mud.Component) error {
		return component.Close(ctx)
	}, mud.All)
	return err
}

// Initialize creates all the requested components.
func Initialize(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) (err error) {
	err = mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		return component.Init(ctx)
	}, mud.All)
	return err
}
