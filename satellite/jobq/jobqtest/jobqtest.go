// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqtest

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/server"
	"storj.io/storj/satellite/repair/queue"
)

// Run runs the given test function with a new test context and a new repair queue.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue)) {
	ctx := testcontext.New(t)

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 1e6)
	require.NoError(t, err)

	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx.Go(func() error {
		return srv.Run(srvCtx)
	})

	cli, err := jobq.Dial(srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cli.Close()) })

	rq := jobq.NewRepairJobQueue(cli)
	t.Cleanup(func() { require.NoError(t, rq.Close()) })

	test(ctx, t, rq)

	cancel()
	ctx.Wait()
}
