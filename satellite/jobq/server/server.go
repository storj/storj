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

	"github.com/spacemonkeygo/monkit/v3"
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

var (
	mon = monkit.Package()
)

// Server represents a job queue DRPC server.
type Server struct {
	log      *zap.Logger
	QueueMap *QueueMap
	endpoint pb.DRPCJobQueueServer
	listener net.Listener
	tlsOpts  *tlsopts.Options
	mux      *drpcmux.Mux
}

// New creates a new Server instance.
func New(log *zap.Logger, listener net.Listener, tlsOpts *tlsopts.Options, retryAfter time.Duration, initAlloc, maxItems, memReleaseThreshold int) (*Server, error) {
	queueFactory := func(placement storj.PlacementConstraint) (*jobqueue.Queue, error) {
		return jobqueue.NewQueue(log.Named(fmt.Sprintf("placement-%d", placement)), retryAfter, initAlloc, maxItems, memReleaseThreshold)
	}
	queueMap := NewQueueMap(log, queueFactory)
	endpoint := NewEndpoint(log, queueMap)
	mux := drpcmux.New()
	err := pb.DRPCRegisterJobQueue(mux, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to register job queue implementation: %w", err)
	}

	return &Server{
		log:      log,
		QueueMap: queueMap,
		endpoint: endpoint,
		listener: listener,
		tlsOpts:  tlsOpts,
		mux:      mux,
	}, nil
}

// Run runs the server (accepting connections on the listener) until the context is canceled.
func (s *Server) Run(ctx context.Context) (err error) {
	mon.Task()(&ctx)(&err)

	// allow connections with or without the DRPC connection header
	lisMux := drpcmigrate.NewListenMux(s.listener, len(drpcmigrate.DRPCHeader))
	drpcWithHeaderListener := lisMux.Route(drpcmigrate.DRPCHeader)
	drpcListener := lisMux.Default()
	tlsConfig := s.tlsOpts.ServerTLSConfig()

	s.log.Info("server listening", zap.Stringer("address", s.listener.Addr()))

	var group errgroup.Group

	group.Go(func() error {
		log := s.log.Named("drpc-with-header")
		srvWithHeader := drpcserver.NewWithOptions(s.mux, drpcserver.Options{
			Log: func(err error) {
				log.Error("accept error", zap.Error(err))
			},
		})
		l := tls.NewListener(drpcWithHeaderListener, tlsConfig)
		return srvWithHeader.Serve(ctx, l)
	})
	group.Go(func() error {
		log := s.log.Named("drpc-no-header")
		srvWithoutHeader := drpcserver.NewWithOptions(s.mux, drpcserver.Options{
			Log: func(err error) {
				log.Error("accept error", zap.Error(err))
			},
		})
		l := tls.NewListener(drpcListener, tlsConfig)
		return srvWithoutHeader.Serve(ctx, l)
	})
	group.Go(func() error {
		return lisMux.Run(ctx)
	})

	err = group.Wait()
	s.QueueMap.StopAll()

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

// SetTimeFunc sets the time function for all queues currently initialized in
// the server. This is primarily used for testing to control the timestamps used
// in the queue.
//
// Importantly, this will not affect queues to be initialized after this point.
func (s *Server) SetTimeFunc(timeFunc func() time.Time) {
	allQueues := s.QueueMap.GetAllQueues()
	for _, q := range allQueues {
		q.Now = timeFunc
	}
}
