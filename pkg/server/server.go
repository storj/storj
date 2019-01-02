// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"
	"time"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/utils"
)

// Service represents a specific gRPC method collection to be registered
// on a shared gRPC server. PointerDB, OverlayCache, PieceStore, Kademlia,
// StatDB, etc. are all examples of services.
type Service interface {
	Run(ctx context.Context, server *Server) error
}

type handle struct {
	grpc *grpc.Server
	lis  net.Listener
}

func (h handle) Serve() error {
	err := h.grpc.Serve(h.lis)
	if err == grpc.ErrServerStopped {
		return nil
	}
	return Error.Wrap(err)
}

func (h handle) Close() error {
	h.grpc.GracefulStop()
	// GracefulStop closes the listener
	return nil
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	public   handle
	private  handle
	next     []Service
	identity *identity.FullIdentity
}

// NewServer creates a Server out of an Identity, public and private
// gRPC handles and listeners, and a set of services. Closing a server will
// stop the grpc servers and close the listeners.
// A public handler is expected to be exposed to the world, whereas the private
// handler is for inspectors and debug tools only.
func NewServer(
	identity *identity.FullIdentity,
	publicSrv *grpc.Server, publicLis net.Listener,
	privateSrv *grpc.Server, privateLis net.Listener,
	services ...Service) *Server {
	return &Server{
		public:   handle{grpc: publicSrv, lis: publicLis},
		private:  handle{grpc: privateSrv, lis: privateLis},
		next:     services,
		identity: identity,
	}
}

// Identity returns the server's identity
func (p *Server) Identity() *identity.FullIdentity { return p.identity }

// Close shuts down the server
func (p *Server) Close() error {
	errch := make(chan error)
	go func() {
		errch <- Error.Wrap(p.public.Close())
	}()
	go func() {
		errch <- Error.Wrap(p.private.Close())
	}()
	return errs.Combine(<-errch, <-errch)
}

// PublicRPC returns a gRPC handle to the public, exposed interface
func (p *Server) PublicRPC() *grpc.Server { return p.public.grpc }

// PrivateRPC returns a gRPC handle to the private, internal interface
func (p *Server) PrivateRPC() *grpc.Server { return p.private.grpc }

// PublicAddr returns the address of the public, exposed interface
func (p *Server) PublicAddr() net.Addr { return p.public.lis.Addr() }

// PrivateAddr returns the address of the private, internal interface
func (p *Server) PrivateAddr() net.Addr { return p.private.lis.Addr() }

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

	errch := make(chan error, 2)
	go func() {
		errch <- p.public.Serve()
	}()
	go func() {
		errch <- p.private.Serve()
	}()
	return utils.CollectErrors(errch, 5*time.Second)
}
