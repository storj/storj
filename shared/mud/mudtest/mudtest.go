// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mudtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

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

// WithTestLogger utility to provide test logger for mud ball.
func WithTestLogger(tb testing.TB, modules func(ball *mud.Ball)) func(ball *mud.Ball) {
	return func(ball *mud.Ball) {
		mud.Provide[*zap.Logger](ball, func() *zap.Logger {
			return zaptest.NewLogger(tb)
		})
		modules(ball)
	}
}
