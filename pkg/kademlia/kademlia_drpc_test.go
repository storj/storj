// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build drpc

package kademlia

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/drpc/drpcserver"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/listenmux"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
)

func newListener(t *testing.T, ctx *testcontext.Context, addr string) (net.Listener, func()) {
	lis, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	listenCtx, cancel := context.WithCancel(ctx)
	mux := listenmux.New(lis, 8)
	ctx.Go(func() error { return mux.Run(listenCtx) })
	return mux.Route("DRPC!!!1"), cancel
}

func testNode(t *testing.T, ctx *testcontext.Context, name string, bn []pb.Node) (*Kademlia, func()) {
	lis, lisCancel := newListener(t, ctx, "127.0.0.1:0")

	fid, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	logger := zaptest.NewLogger(t)
	k, err := newKademlia(logger, pb.NodeType_STORAGE, bn, lis.Addr().String(), pb.NodeOperator{}, fid, defaultAlpha)
	require.NoError(t, err)
	s := NewEndpoint(logger, k, nil, k.routingTable, nil)

	tlsOptions, err := tlsopts.NewOptions(fid, tlsopts.Config{PeerIDVersions: "*"}, nil)
	require.NoError(t, err)

	tlsLis := tls.NewListener(lis, tlsOptions.ServerTLSConfig())
	drpcServer := drpcserver.New()
	pb.DRPCRegisterNodes(drpcServer, s)
	serveCtx, cancel := context.WithCancel(ctx)
	ctx.Go(func() error { return drpcServer.Serve(serveCtx, tlsLis) })

	return k, func() {
		cancel()
		lisCancel()
		assert.NoError(t, k.Close())
	}
}

func startTestNodeServer(t *testing.T, ctx *testcontext.Context) (*mockNodesServer, *identity.FullIdentity, string, func()) {
	lis, lisCancel := newListener(t, ctx, "127.0.0.1:0")

	ca, err := testidentity.NewTestCA(ctx)
	require.NoError(t, err)

	fullIdentity, err := ca.NewIdentity()
	require.NoError(t, err)

	tlsOptions, err := tlsopts.NewOptions(fullIdentity, tlsopts.Config{}, nil)
	require.NoError(t, err)

	tlsLis := tls.NewListener(lis, tlsOptions.ServerTLSConfig())
	drpcServer := drpcserver.New()
	mn := &mockNodesServer{queryCalled: 0}
	pb.DRPCRegisterNodes(drpcServer, mn)
	serveCtx, cancel := context.WithCancel(context.Background())
	ctx.Go(func() error { return drpcServer.Serve(serveCtx, tlsLis) })

	return mn, fullIdentity, lis.Addr().String(), func() {
		cancel()
		lisCancel()
	}
}

func newTestServer(t *testing.T, ctx *testcontext.Context, lis net.Listener) (*mockNodesServer, func()) {
	ca, err := testidentity.NewTestCA(ctx)
	require.NoError(t, err)

	fullIdentity, err := ca.NewIdentity()
	require.NoError(t, err)

	tlsOptions, err := tlsopts.NewOptions(fullIdentity, tlsopts.Config{}, nil)
	require.NoError(t, err)

	tlsLis := tls.NewListener(lis, tlsOptions.ServerTLSConfig())
	drpcServer := drpcserver.New()
	mn := &mockNodesServer{queryCalled: 0}
	pb.DRPCRegisterNodes(drpcServer, mn)
	serveCtx, cancel := context.WithCancel(context.Background())
	ctx.Go(func() error { return drpcServer.Serve(serveCtx, tlsLis) })

	return mn, cancel
}
