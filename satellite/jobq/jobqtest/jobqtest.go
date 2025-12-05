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
	"storj.io/common/memory"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/repair/queue"
)

// ServerOptions represents the options to be used for creating a new server.
type ServerOptions struct {
	Host                string
	Identity            *identity.FullIdentity
	TLS                 tlsopts.Config
	RetryAfter          time.Duration
	MaxMemPerPlacement  memory.Size
	InitAlloc           memory.Size
	MemReleaseThreshold memory.Size

	// Timeout for the testcontext.
	Timeout time.Duration
}

// TestServer wraps a jobq server together with its setup information, so that
// tests can easily connect to the server.
type TestServer struct {
	*satellite.JobqServer
	// Identity is the identity to be used to connect to the server (not
	// necessarily the same as the identity used by the server for accepting
	// connections).
	Identity *identity.FullIdentity
	// TLSOpts is the TLS options to be used to connect to the server (not
	// necessarily the same as the TLS options used by the server for accepting
	// connections).
	TLSOpts *tlsopts.Options
	// NodeURL is the NodeURL to be used to connect to the server.
	NodeURL storj.NodeURL
}

// SetTimeFunc sets the time function for all queues currently initialized in
// the server. This is primarily used for testing to control the timestamps used
// in the queue.
//
// Importantly, this will not affect queues to be initialized after this point.
func (ts *TestServer) SetTimeFunc(timeFunc func() time.Time) {
	for _, q := range ts.JobqServer.Jobq.QueueMap.GetAllQueues() {
		q.Now = timeFunc
	}
}

// WithServer runs the given function with a new test context and a new jobq
// server. The config for connecting to the server is given to the test
// function. The server is shut down when the function returns.
func WithServer(t *testing.T, options *ServerOptions, f func(ctx *testcontext.Context, server *TestServer)) {
	log := zaptest.NewLogger(t)

	if options == nil {
		options = &ServerOptions{}
	}
	if options.Identity == nil {
		ident, err := identity.NewFullIdentity(t.Context(), identity.NewCAOptions{})
		require.NoError(t, err)
		options.Identity = ident
	}
	if options.TLS == (tlsopts.Config{}) {
		options.TLS = tlsopts.Config{
			PeerIDVersions: "latest",
		}
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
	if options.Timeout == 0 {
		options.Timeout = testcontext.DefaultTimeout
	}

	host := options.Host
	if host == "" {
		host = "127.0.0.1"
	}

	peer, err := satellite.NewJobq(log, options.Identity, nil, &satellite.JobqConfig{
		ListenAddress:       net.JoinHostPort(host, "0"),
		TLS:                 options.TLS,
		InitAlloc:           options.InitAlloc,
		MaxMemPerPlacement:  options.MaxMemPerPlacement,
		MemReleaseThreshold: options.MemReleaseThreshold,
		RetryAfter:          options.RetryAfter,
	}, nil)
	require.NoError(t, err)
	clientOpts, err := tlsopts.NewOptions(options.Identity, options.TLS, nil)
	require.NoError(t, err)
	testSrv := &TestServer{
		JobqServer: peer,
		Identity:   options.Identity,
		TLSOpts:    clientOpts,
		NodeURL: storj.NodeURL{
			Address: peer.Jobq.Server.Addr().String(),
			ID:      options.Identity.ID,
		},
	}

	var group errgroup.Group
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	group.Go(func() error {
		err := testSrv.Run(ctx)
		return err
	})

	testmonkit.Run(ctx, t, func(parent context.Context) {
		pprof.Do(parent, pprof.Labels("test", t.Name()), func(parent context.Context) {
			ctx := testcontext.NewWithContextAndTimeout(parent, t, options.Timeout)
			defer ctx.Cleanup()
			f(ctx, testSrv)
		})
	})

	cancel()
	require.NoError(t, group.Wait())
}

// WithServerAndClient runs the given function with a new test context, a new
// jobq server, and a jobq client connected to that server.
func WithServerAndClient(t *testing.T, sOpts *ServerOptions, f func(ctx *testcontext.Context, srv *TestServer, cli *jobq.Client)) {
	WithServer(t, sOpts, func(ctx *testcontext.Context, srv *TestServer) {
		dialer := jobq.NewDialer(srv.TLSOpts)
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
