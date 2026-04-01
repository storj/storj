// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mudtest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/mud"
)

// Run executes mud test or benchmark with the give module, creating (and closing) Target and all transitive dependencies.
func Run[Target any, TB testing.TB](tb TB, modules func(ball *mud.Ball), testRun func(ctx context.Context, tb TB, target Target)) {
	ctx := testcontext.New(tb)
	ball := mud.NewBall()
	mud.Supply[testing.TB](ball, tb)
	modules(ball)
	for _, component := range mud.FindSelectedWithDependencies(ball, mud.Select[Target](ball)) {
		err := component.Init(ctx)
		require.NoError(tb, err)
	}
	defer func() {
		for _, component := range mud.FindSelectedWithDependencies(ball, mud.Select[Target](ball)) {
			err := component.Close(ctx)
			assert.NoError(tb, err)
		}
	}()
	target := mud.MustLookup[Target](ball)
	testRun(ctx, tb, target)
}

// RunF executes function with initializing all required methods.
func RunF[TB testing.TB](tb TB, modules func(ball *mud.Ball), selector mud.ComponentSelector, testRun any) {
	ctx, cancel := context.WithCancel(testcontext.New(tb))
	defer cancel()
	ball := mud.NewBall()
	mud.Supply[testing.TB](ball, tb)
	modules(ball)

	err := mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		err := component.Init(ctx)
		require.NoError(tb, err)
		return nil
	})
	require.NoError(tb, err)

	g, ctx := errgroup.WithContext(ctx)

	defer func() {
		err := mud.ForEachDependencyReverse(ball, selector, func(component *mud.Component) error {
			err := component.Close(ctx)
			require.NoError(tb, err)
			return nil
		})
		require.NoError(tb, err)
	}()

	err = mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		g.Go(func() error {
			err := component.Run(ctx, g)
			return err
		})
		return nil
	})
	require.NoError(tb, err)

	err = mud.Execute0(ctx, ball, testRun)
	require.NoError(tb, err)

	cancel()
	err = g.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		require.NoError(tb, err)
	}
}

// WithTestLogger utility to provide test logger for mud ball.
func WithTestLogger(tb testing.TB, modules func(ball *mud.Ball)) func(ball *mud.Ball) {
	return func(ball *mud.Ball) {
		mud.Provide[*zap.Logger](ball, func() *zap.Logger {
			return zaptest.NewLogger(tb)
		})
		modules(ball)
	}
}
