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

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/server"
	"storj.io/storj/satellite/repair/queue"
)

// ServerOptions represents the options to be used for creating a new server.
type ServerOptions struct {
	Identity            *identity.FullIdentity
	TLSOpts             *tlsopts.Options
	RetryAfter          time.Duration
	InitAlloc           int
	MaxItems            int
	MemReleaseThreshold int
}

// TestServer wraps a jobq server together with its setup information, so that
// tests can easily connect to the server.
type TestServer struct {
	*server.Server
	Identity *identity.FullIdentity
	TLSOpts  *tlsopts.Options
	NodeURL  storj.NodeURL
}

// WithServer runs the given function with a new test context and a new jobq
// server. The config for connecting to the server is given to the test
// function. The server is shut down when the function returns.
func WithServer(t *testing.T, options *ServerOptions, f func(ctx *testcontext.Context, server *TestServer)) {
	log := zaptest.NewLogger(t)
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	listener, err := net.Listen(addr.Network(), addr.String())
	require.NoError(t, err)

	if options == nil {
		options = &ServerOptions{}
	}
	if options.Identity == nil {
		ident, err := identity.NewFullIdentity(context.Background(), identity.NewCAOptions{})
		require.NoError(t, err)
		options.Identity = ident
	}
	if options.TLSOpts == nil {
		options.TLSOpts, err = tlsopts.NewOptions(options.Identity, tlsopts.Config{
			PeerIDVersions: "latest",
		}, nil)
		require.NoError(t, err)
	}
	if options.RetryAfter == 0 {
		options.RetryAfter = time.Hour
	}
	if options.InitAlloc == 0 {
		options.InitAlloc = 1e8
	}
	if options.MemReleaseThreshold == 0 {
		options.MemReleaseThreshold = 1e6
	}

	srv, err := server.New(log, listener, options.TLSOpts, options.RetryAfter, options.InitAlloc, options.MaxItems, options.MemReleaseThreshold)
	require.NoError(t, err)
	testSrv := &TestServer{
		Server:   srv,
		Identity: options.Identity,
		TLSOpts:  options.TLSOpts,
		NodeURL: storj.NodeURL{
			Address: srv.Addr().String(),
			ID:      options.Identity.ID,
		},
	}

	var group errgroup.Group
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	group.Go(func() error {
		err := testSrv.Run(ctx)
		return err
	})

	testmonkit.Run(ctx, t, func(parent context.Context) {
		pprof.Do(parent, pprof.Labels("test", t.Name()), func(parent context.Context) {
			f(testcontext.New(t), testSrv)
		})
	})

	cancel()
	require.NoError(t, group.Wait())
}

// WithServerAndClient runs the given function with a new test context, a new
// jobq server, and a jobq client connected to that server.
func WithServerAndClient(t *testing.T, sOpts *ServerOptions, f func(ctx *testcontext.Context, srv *TestServer, cli *jobq.Client)) {
	WithServer(t, sOpts, func(ctx *testcontext.Context, srv *TestServer) {
		dialer := rpc.NewDefaultPooledDialer(srv.TLSOpts)
		conn, err := dialer.DialNodeURL(ctx, srv.NodeURL)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })

		cli := jobq.WrapConn(conn)

		f(ctx, srv, cli)
	})
}

// Run runs the given test function with a new test context and a new repair
// queue connected to a running jobq server. The server is shut down when the
// function returns.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue)) {
	WithServerAndClient(t, nil, func(ctx *testcontext.Context, srv *TestServer, cli *jobq.Client) {
		rq := jobq.WrapJobQueue(cli)
		test(ctx, t, rq)
	})
}
