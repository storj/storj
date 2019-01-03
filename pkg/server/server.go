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

// Handle is a type that pairs a gRPC server to a listener
type Handle struct {
	Srv *grpc.Server
	Lis net.Listener
}

// NewHandle constructs a gRPC/listener handle
func NewHandle(srv *grpc.Server, lis net.Listener) *Handle {
	return &Handle{Srv: srv, Lis: lis}
}

// Serve calls Serve on the gRPC server with the handle's listener
func (h *Handle) Serve() error {
	err := h.Srv.Serve(h.Lis)
	if err == grpc.ErrServerStopped {
		return nil
	}
	return Error.Wrap(err)
}

// Close closes the gRPC server gracefully and shuts down the listener.
func (h *Handle) Close() error {
	h.Srv.GracefulStop()
	_ = h.Lis.Close()
	return nil
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	public   *Handle
	private  *Handle
	next     []Service
	identity *identity.FullIdentity
}

// NewServer creates a Server out of an Identity, public and private
// gRPC handles and listeners, and a set of services. Closing a server will
// stop the grpc servers and close the listeners.
// A public handler is expected to be exposed to the world, whereas the private
// handler is for inspectors and debug tools only.
func NewServer(identity *identity.FullIdentity, public, private *Handle, services ...Service) *Server {
	return &Server{
		public:   public,
		private:  private,
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
func (p *Server) PublicRPC() *grpc.Server { return p.public.Srv }

// PrivateRPC returns a gRPC handle to the private, internal interface
func (p *Server) PrivateRPC() *grpc.Server { return p.private.Srv }

// PublicAddr returns the address of the public, exposed interface
func (p *Server) PublicAddr() net.Addr { return p.public.Lis.Addr() }

// PrivateAddr returns the address of the private, internal interface
func (p *Server) PrivateAddr() net.Addr { return p.private.Lis.Addr() }

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

	return utils.RunJointly(5*time.Second, p.private.Serve, p.public.Serve)
}
