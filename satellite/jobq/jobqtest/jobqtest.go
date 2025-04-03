// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqtest

import (
	"context"
	"net"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/server"
	"storj.io/storj/satellite/repair/queue"
)

// WithServer runs the given function with a new test context and a new jobq
// server. The config for connecting to the server is given to the test
// function. The server is shut down when the function returns.
func WithServer(t *testing.T, f func(ctx *testcontext.Context, config jobq.Config)) {
	ctx := testcontext.New(t)

	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv, err := server.New(log, addr, nil, time.Hour, 1e8, 0, 1e6)
	require.NoError(t, err)

	var group errgroup.Group
	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	group.Go(func() error {
		err := srv.Run(srvCtx)
		return err
	})

	config := jobq.Config{
		ServerNodeURL: storj.NodeURL{
			Address: srv.Addr().String(),
		},
	}

	testmonkit.Run(ctx, t, func(parent context.Context) {
		pprof.Do(parent, pprof.Labels("test", t.Name()), func(parent context.Context) {
			f(testcontext.NewWithContext(parent, t), config)
		})
	})

	cancel()
	require.NoError(t, group.Wait())
}

// Run runs the given test function with a new test context and a new repair
// queue connected to a running jobq server. The server is shut down when the
// function returns.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue)) {
	WithServer(t, func(ctx *testcontext.Context, config jobq.Config) {
		rq, err := jobq.OpenJobQueue(ctx, nil, config)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, rq.Close()) })

		test(ctx, t, rq)
	})
}
