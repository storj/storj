// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/drpc/drpcmigrate"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	pb "storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/jobq/jobqueue"
)

// Server represents a job queue DRPC server.
type Server struct {
	log      *zap.Logger
	queueMap *QueueMap
	endpoint pb.DRPCJobQueueServer
	listener net.Listener
	mux      *drpcmux.Mux
}

// New creates a new Server instance.
func New(log *zap.Logger, listenAddress net.Addr, tlsOpts *tlsopts.Options, retryAfter time.Duration, initAlloc, memReleaseThreshold int) (*Server, error) {
	queueFactory := func(placement storj.PlacementConstraint) (*jobqueue.Queue, error) {
		return jobqueue.NewQueue(log.Named(fmt.Sprintf("placement-%d", placement)), retryAfter, initAlloc, memReleaseThreshold)
	}
	queueMap := NewQueueMap(log, queueFactory)
	endpoint := NewEndpoint(log, queueMap)
	mux := drpcmux.New()
	err := pb.DRPCRegisterJobQueue(mux, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to register job queue implementation: %w", err)
	}

	listener, err := net.Listen(listenAddress.Network(), listenAddress.String())
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %q: %w", listenAddress, err)
	}

	if tlsOpts != nil {
		listener = tls.NewListener(listener, tlsOpts.ServerTLSConfig())
	}

	return &Server{
		log:      log,
		queueMap: queueMap,
		endpoint: endpoint,
		listener: listener,
		mux:      mux,
	}, nil
}

// Run runs the server (accepting connections on the listener) until the context is canceled.
func (s *Server) Run(ctx context.Context) error {
	// allow connections with or without the DRPC connection header
	lisMux := drpcmigrate.NewListenMux(s.listener, len(drpcmigrate.DRPCHeader))
	drpcWithHeaderListener := lisMux.Route(drpcmigrate.DRPCHeader)
	drpcListener := lisMux.Default()

	s.log.Info("server listening", zap.Stringer("address", s.listener.Addr()))

	var group errgroup.Group

	group.Go(func() error {
		srvWithHeader := drpcserver.New(s.mux)
		return srvWithHeader.Serve(ctx, drpcWithHeaderListener)
	})
	group.Go(func() error {
		srvWithoutHeader := drpcserver.New(s.mux)
		return srvWithoutHeader.Serve(ctx, drpcListener)
	})
	group.Go(func() error {
		return lisMux.Run(ctx)
	})

	err := group.Wait()
	s.queueMap.StopAll()

	if err == nil || errors.Is(err, context.Canceled) {
		s.log.Info("server closed", zap.Stringer("address", s.listener.Addr()))
		return nil
	}
	return fmt.Errorf("server failed: %w", err)
}

// Addr returns the address on which the server is listening.
func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}
