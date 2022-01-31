// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package lifecycle_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/private/lifecycle"
)

func TestGroup(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	closed := []string{}
	var astart, cstart bool

	group := lifecycle.NewGroup(log)
	group.Add(lifecycle.Item{
		Name: "A",
		Run: func(ctx context.Context) error {
			astart = true
			log.Info("Run A")
			return nil
		},
		Close: func() error {
			closed = append(closed, "A")
			return nil
		},
	})
	group.Add(lifecycle.Item{
		Name: "B",
		Run:  nil,
		Close: func() error {
			closed = append(closed, "B")
			return nil
		},
	})
	group.Add(lifecycle.Item{
		Name: "C",
		Run: func(ctx context.Context) error {
			cstart = true
			log.Info("Run C")
			return nil
		},
		Close: nil,
	})

	g, gctx := errgroup.WithContext(ctx)
	group.Run(gctx, g)

	err := g.Wait()
	require.NoError(t, err)

	require.True(t, astart)
	require.True(t, cstart)

	err = group.Close()
	require.NoError(t, err)

	require.Equal(t, []string{"B", "A"}, closed)
}
