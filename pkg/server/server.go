// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
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
	publicListener  net.Listener
	privateListener net.Listener
	grpc            *grpc.Server
	next            []Service
	identity        *identity.FullIdentity
}

// New creates a Server out of an Identity, a net.Listener,
// a UnaryServerInterceptor, and a set of services.
func New(opts *tlsopts.Options, pubAddr, privAddr string, interceptor grpc.UnaryServerInterceptor, services ...Service) (*Server, error) {
	unaryInterceptor := unaryInterceptor
	if interceptor != nil {
		unaryInterceptor = combineInterceptors(unaryInterceptor, interceptor)
	}

	pubLis, err := net.Listen("tcp", pubAddr)
	if err != nil {
		return nil, err
	}

	privLis, err := net.Listen("tcp", privAddr)
	if err != nil {
		return nil, err
	}

	return &Server{
		publicListener:  pubLis,
		privateListener: privLis,
		grpc: grpc.NewServer(
			grpc.StreamInterceptor(streamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			opts.ServerOption(),
		),
		next:     services,
		identity: opts.Ident,
	}, nil
}

// Identity returns the server's identity
func (p *Server) Identity() *identity.FullIdentity { return p.identity }

// PublicAddr returns the server's listener address
func (p *Server) PublicAddr() net.Addr { return p.publicListener.Addr() }

// PrivateAddr returns the server's listener address
func (p *Server) PrivateAddr() net.Addr { return p.privateListener.Addr() }

// PublicListener returns the server's public listener
func (p *Server) PublicListener() net.Listener { return p.publicListener }

// PrivateListener returns the server's private listener
func (p *Server) PrivateListener() net.Listener { return p.privateListener }

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

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return p.Close()
	})
	group.Go(func() error {
		defer cancel()
		return p.grpc.Serve(p.publicListener)
	})
	group.Go(func() error {
		defer cancel()
		return p.grpc.Serve(p.privateListener)
	})

	return group.Wait()
}
