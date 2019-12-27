// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/drpc/drpcserver"
	"storj.io/storj/pkg/listenmux"
)

// Service represents a specific gRPC method collection to be registered
// on a shared gRPC server. Metainfo, OverlayCache, PieceStore,
// etc. are all examples of services.
type Service interface {
	Run(ctx context.Context, server *Server) error
}

type public struct {
	listener net.Listener
	drpc     *drpcserver.Server
	grpc     *grpc.Server
}

type private struct {
	listener net.Listener
	drpc     *drpcserver.Server
	grpc     *grpc.Server
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	log        *zap.Logger
	public     public
	private    private
	next       []Service
	tlsOptions *tlsopts.Options

	mu   sync.Mutex
	wg   sync.WaitGroup
	once sync.Once
	done chan struct{}
}

// New creates a Server out of an Identity, a net.Listener,
// a UnaryServerInterceptor, and a set of services.
func New(log *zap.Logger, tlsOptions *tlsopts.Options, publicAddr, privateAddr string, interceptor grpc.UnaryServerInterceptor, services ...Service) (*Server, error) {
	server := &Server{
		log:        log,
		next:       services,
		tlsOptions: tlsOptions,
		done:       make(chan struct{}),
	}

	serverOptions := drpcserver.Options{
		Manager: rpc.NewDefaultManagerOptions(),
	}

	unaryInterceptor := server.logOnErrorUnaryInterceptor
	if interceptor != nil {
		unaryInterceptor = CombineInterceptors(unaryInterceptor, interceptor)
	}

	publicListener, err := net.Listen("tcp", publicAddr)
	if err != nil {
		return nil, err
	}
	server.public = public{
		listener: publicListener,
		drpc:     drpcserver.NewWithOptions(serverOptions),
		grpc: grpc.NewServer(
			grpc.StreamInterceptor(server.logOnErrorStreamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			tlsOptions.ServerOption(),
		),
	}

	privateListener, err := net.Listen("tcp", privateAddr)
	if err != nil {
		return nil, errs.Combine(err, publicListener.Close())
	}
	server.private = private{
		listener: privateListener,
		drpc:     drpcserver.NewWithOptions(serverOptions),
		grpc:     grpc.NewServer(),
	}

	return server, nil
}

// Identity returns the server's identity
func (p *Server) Identity() *identity.FullIdentity { return p.tlsOptions.Ident }

// Addr returns the server's public listener address
func (p *Server) Addr() net.Addr { return p.public.listener.Addr() }

// PrivateAddr returns the server's private listener address
func (p *Server) PrivateAddr() net.Addr { return p.private.listener.Addr() }

// GRPC returns the server's gRPC handle for registration purposes
func (p *Server) GRPC() *grpc.Server { return p.public.grpc }

// DRPC returns the server's dRPC handle for registration purposes
func (p *Server) DRPC() *drpcserver.Server { return p.public.drpc }

// PrivateGRPC returns the server's gRPC handle for registration purposes
func (p *Server) PrivateGRPC() *grpc.Server { return p.private.grpc }

// PrivateDRPC returns the server's dRPC handle for registration purposes
func (p *Server) PrivateDRPC() *drpcserver.Server { return p.private.drpc }

// Close shuts down the server
func (p *Server) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close done and wait for any Runs to exit.
	p.once.Do(func() { close(p.done) })
	p.wg.Wait()

	// Ensure the listeners are closed in case Run was never called.
	// We ignore these errors because there's not really anything to do
	// even if they happen, and they'll just be errors due to duplicate
	// closes anyway.
	_ = p.public.listener.Close()
	_ = p.private.listener.Close()
	return nil
}

// Run will run the server and all of its services
func (p *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// are there any unstarted services? start those first. the
	// services should know to call Run again once they're ready.
	if len(p.next) > 0 {
		next := p.next[0]
		p.next = p.next[1:]
		return next.Run(ctx, p)
	}

	// Make sure the server isn't already closed. If it is, register
	// ourselves in the wait group so that Close can wait on it.
	p.mu.Lock()
	select {
	case <-p.done:
		p.mu.Unlock()
		return errs.New("server closed")
	default:
		p.wg.Add(1)
		defer p.wg.Done()
	}
	p.mu.Unlock()

	// We want to launch the muxes in a different group so that they are
	// only closed after we're sure that p.Close is called. The reason why
	// is so that we don't get "listener closed" errors because the
	// Run call exits and closes the listeners before the servers have had
	// a chance to be notified that they're done running.
	const drpcHeader = "DRPC!!!1"

	publicMux := listenmux.New(p.public.listener, len(drpcHeader))
	publicDRPCListener := tls.NewListener(publicMux.Route(drpcHeader), p.tlsOptions.ServerTLSConfig())

	privateMux := listenmux.New(p.private.listener, len(drpcHeader))
	privateDRPCListener := privateMux.Route(drpcHeader)

	// We need a new context chain because we require this context to be
	// canceled only after all of the upcoming grpc/drpc servers have
	// fully exited. The reason why is because Run closes listener for
	// the mux when it exits, and we can only do that after all of the
	// Servers are no longer accepting.
	muxCtx, muxCancel := context.WithCancel(context.Background())
	defer muxCancel()

	var muxGroup errgroup.Group
	muxGroup.Go(func() error {
		return publicMux.Run(muxCtx)
	})
	muxGroup.Go(func() error {
		return privateMux.Run(muxCtx)
	})

	// Now we launch all the stuff that uses the listeners.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var group errgroup.Group
	group.Go(func() error {
		select {
		case <-p.done:
			cancel()
		case <-ctx.Done():
		}

		p.public.grpc.GracefulStop()
		p.private.grpc.GracefulStop()

		return nil
	})

	group.Go(func() error {
		defer cancel()
		return p.public.grpc.Serve(publicMux.Default())
	})
	group.Go(func() error {
		defer cancel()
		return p.public.drpc.Serve(ctx, publicDRPCListener)
	})
	group.Go(func() error {
		defer cancel()
		return p.private.grpc.Serve(privateMux.Default())
	})
	group.Go(func() error {
		defer cancel()
		return p.private.drpc.Serve(ctx, privateDRPCListener)
	})

	// Now we wait for all the stuff using the listeners to exit.
	err = group.Wait()

	// Now we close down our listeners.
	muxCancel()
	return errs.Combine(err, muxGroup.Wait())
}
