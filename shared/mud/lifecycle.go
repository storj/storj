// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// RunWithDependencies will init and run all components which are matched by the selector.
func RunWithDependencies(ctx context.Context, ball *Ball, selector ComponentSelector) error {
	log := ball.getLogger()
	return runComponents(ctx, log, FindSelectedWithDependencies(ball, selector))
}

// Run runs the required component and all dependencies in the right order.
func runComponents(ctx context.Context, log *zap.Logger, components []*Component) error {
	err := forEachComponent(components, func(component *Component) error {
		log.Info("init", zap.String("component", component.Name()))
		return component.Init(ctx)
	})
	if err != nil {
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	err = forEachComponent(components, func(component *Component) error {
		log.Info("init", zap.String("starting", component.Name()))
		return component.Run(ctx, g)
	})
	if err != nil {
		return err
	}
	return g.Wait()
}

// CloseAll calls the close callback stage on all initialized components.
func CloseAll(ball *Ball, timeout time.Duration) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	components := ball.registry
	reverse(components)
	log := ball.getLogger()
	return forEachComponent(components, func(component *Component) error {
		log.Info("closing", zap.String("component", component.Name()))
		if component.instance != nil {
			return component.Close(ctx)
		}
		return nil
	})
}

// Reverse reverses the elements of the slice in place.
// TODO: use slices.Reverse when minimum golang version is updated.
func reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
