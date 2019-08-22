// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

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
	listener net.Listener
	grpc     *grpc.Server
}

type private struct {
	listener net.Listener
	grpc     *grpc.Server
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	log      *zap.Logger
	public   public
	private  private
	next     []Service
	identity *identity.FullIdentity
}

// New creates a Server out of an Identity, a net.Listener,
// a UnaryServerInterceptor, and a set of services.
func New(log *zap.Logger, opts *tlsopts.Options, publicAddr, privateAddr string, interceptor grpc.UnaryServerInterceptor, services ...Service) (*Server, error) {
	server := &Server{
		log:      log,
		next:     services,
		identity: opts.Ident,
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
		grpc: grpc.NewServer(
			grpc.StreamInterceptor(server.logOnErrorStreamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			opts.ServerOption(),
		),
	}

	privateListener, err := net.Listen("tcp", privateAddr)
	if err != nil {
		return nil, errs.Combine(err, publicListener.Close())
	}
	server.private = private{
		listener: privateListener,
		grpc:     grpc.NewServer(),
	}

	return server, nil
}

// Identity returns the server's identity
func (p *Server) Identity() *identity.FullIdentity { return p.identity }

// Addr returns the server's public listener address
func (p *Server) Addr() net.Addr { return p.public.listener.Addr() }

// PrivateAddr returns the server's private listener address
func (p *Server) PrivateAddr() net.Addr { return p.private.listener.Addr() }

// GRPC returns the server's gRPC handle for registration purposes
func (p *Server) GRPC() *grpc.Server { return p.public.grpc }

// PrivateGRPC returns the server's gRPC handle for registration purposes
func (p *Server) PrivateGRPC() *grpc.Server { return p.private.grpc }

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
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return p.Close()
	})
	group.Go(func() error {
		defer cancel()
		return p.public.grpc.Serve(p.public.listener)
	})
	group.Go(func() error {
		defer cancel()
		return p.private.grpc.Serve(p.private.listener)
	})

	return group.Wait()
}
