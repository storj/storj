// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
)

// Service represents a specific gRPC method collection to be registered
// on a shared gRPC server. PointerDB, OverlayCache, PieceStore, Kademlia,
// StatDB, etc. are all examples of services.
type Service interface {
	Run(ctx context.Context, server *Server) error
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	lis      net.Listener
	grpc     *grpc.Server
	next     []Service
	identity *identity.FullIdentity
}

// NewServer creates a Server out of an Identity, a net.Listener,
// a UnaryServerInterceptor, and a set of services.
func NewServer(opts *ServerOptions, lis net.Listener,
	interceptor grpc.UnaryServerInterceptor, services ...Service) (
	*Server, error) {
	grpcOpts, err := opts.GRPCOpts()
	if err != nil {
		return nil, err
	}

	unaryInterceptor := unaryInterceptor
	if interceptor != nil {
		unaryInterceptor = combineInterceptors(unaryInterceptor, interceptor)
	}

	return &Server{
		lis: lis,
		grpc: grpc.NewServer(
			grpc.StreamInterceptor(streamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			grpcOpts,
		),
		next:     services,
		identity: opts.Ident,
	}, nil
}

// Identity returns the server's identity
func (p *Server) Identity() *identity.FullIdentity { return p.identity }

// Addr returns the server's listener address
func (p *Server) Addr() net.Addr { return p.lis.Addr() }

// GRPC returns the server's gRPC handle for registration purposes
func (p *Server) GRPC() *grpc.Server { return p.grpc }

// Close shuts down the server
func (p *Server) Close() error {
	p.grpc.GracefulStop()
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

	return p.grpc.Serve(p.lis)
}
