// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/storj/drpc/drpcmux"
	"storj.io/storj/drpc/drpcserver"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
)

// Service represents a specific gRPC method collection to be registered
// on a shared gRPC server. Metainfo, OverlayCache, PieceStore, Kademlia,
// etc. are all examples of services.
type Service interface {
	Run(ctx context.Context, server *Server) error
}

type public struct {
	mux  *drpcmux.Mux
	grpc *grpc.Server
	drpc *drpcserver.Server
}

type private struct {
	mux  *drpcmux.Mux
	grpc *grpc.Server
	drpc *drpcserver.Server
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	log      *zap.Logger
	public   public
	private  private
	next     []Service
	identity *identity.FullIdentity
	opts     *tlsopts.Options
}

// New creates a Server out of an Identity, a net.Listener,
// a UnaryServerInterceptor, and a set of services.
func New(log *zap.Logger, opts *tlsopts.Options, publicAddr, privateAddr string, interceptor grpc.UnaryServerInterceptor, services ...Service) (*Server, error) {
	server := &Server{
		log:      log,
		next:     services,
		identity: opts.Ident,
		opts:     opts,
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
		mux: drpcmux.New(publicListener, 8),
		grpc: grpc.NewServer(
			grpc.StreamInterceptor(server.logOnErrorStreamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			opts.ServerOption(),
		),
		drpc: drpcserver.New(),
	}

	privateListener, err := net.Listen("tcp", privateAddr)
	if err != nil {
		return nil, errs.Combine(err, publicListener.Close())
	}
	server.private = private{
		mux:  drpcmux.New(privateListener, 8),
		grpc: grpc.NewServer(),
		drpc: drpcserver.New(),
	}

	return server, nil
}

// Identity returns the server's identity
func (p *Server) Identity() *identity.FullIdentity { return p.identity }

// Addr returns the server's public listener address
func (p *Server) Addr() net.Addr { return p.public.mux.Default().Addr() }

// PrivateAddr returns the server's private listener address
func (p *Server) PrivateAddr() net.Addr { return p.private.mux.Default().Addr() }

// GRPC returns the server's gRPC handle for registration purposes
func (p *Server) GRPC() *grpc.Server { return p.public.grpc }

func (p *Server) DRPC() *drpcserver.Server { return p.public.drpc }

// PrivateGRPC returns the server's gRPC handle for registration purposes
func (p *Server) PrivateGRPC() *grpc.Server { return p.private.grpc }

func (p *Server) PrivateDRPC() *drpcserver.Server { return p.private.drpc }

// Close shuts down the server
func (p *Server) Close() error {
	p.public.grpc.GracefulStop()
	p.private.grpc.GracefulStop()
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	drpcPublic := tls.NewListener(p.public.mux.Route("drpc!!!1"), p.opts.ServerTLSConfig())
	drpcPrivate := p.private.mux.Route("drpc!!!1")

	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return p.Close()
	})
	group.Go(func() error {
		defer cancel()
		return p.public.grpc.Serve(p.public.mux.Default())
	})
	group.Go(func() error {
		defer cancel()
		return p.public.drpc.Serve(ctx, drpcPublic)
	})
	group.Go(func() error {
		defer cancel()
		return p.private.grpc.Serve(p.private.mux.Default())
	})
	group.Go(func() error {
		defer cancel()
		return p.private.drpc.Serve(ctx, drpcPrivate)
	})
	group.Go(func() error {
		defer cancel()
		return p.public.mux.Run(ctx)
	})
	group.Go(func() error {
		defer cancel()
		return p.private.mux.Run(ctx)
	})
	return group.Wait()
}
