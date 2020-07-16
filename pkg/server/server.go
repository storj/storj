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

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpctracing"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	jaeger "storj.io/monkit-jaeger"
	"storj.io/storj/pkg/listenmux"
)

// Config holds server specific configuration parameters.
type Config struct {
	tlsopts.Config
	Address        string `user:"true" help:"public address to listen on" default:":7777"`
	PrivateAddress string `user:"true" help:"private address to listen on" default:"127.0.0.1:7778"`

	DebugLogTraffic bool `hidden:"true" default:"false"` // Deprecated
}

type public struct {
	listener net.Listener
	drpc     *drpcserver.Server
	mux      *drpcmux.Mux
}

type private struct {
	listener net.Listener
	drpc     *drpcserver.Server
	mux      *drpcmux.Mux
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	log        *zap.Logger
	public     public
	private    private
	tlsOptions *tlsopts.Options

	mu   sync.Mutex
	wg   sync.WaitGroup
	once sync.Once
	done chan struct{}
}

// New creates a Server out of an Identity, a net.Listener,
// and interceptors.
func New(log *zap.Logger, tlsOptions *tlsopts.Options, publicAddr, privateAddr string) (*Server, error) {
	server := &Server{
		log:        log,
		tlsOptions: tlsOptions,
		done:       make(chan struct{}),
	}

	serverOptions := drpcserver.Options{
		Manager: rpc.NewDefaultManagerOptions(),
	}

	publicListener, err := net.Listen("tcp", publicAddr)
	if err != nil {
		return nil, err
	}

	publicMux := drpcmux.New()
	publicTracingHandler := rpctracing.NewHandler(publicMux, jaeger.RemoteTraceHandler)
	server.public = public{
		listener: wrapListener(publicListener),
		drpc:     drpcserver.NewWithOptions(publicTracingHandler, serverOptions),
		mux:      publicMux,
	}

	privateListener, err := net.Listen("tcp", privateAddr)
	if err != nil {
		return nil, errs.Combine(err, publicListener.Close())
	}
	privateMux := drpcmux.New()
	privateTracingHandler := rpctracing.NewHandler(privateMux, jaeger.RemoteTraceHandler)
	server.private = private{
		listener: wrapListener(privateListener),
		drpc:     drpcserver.NewWithOptions(privateTracingHandler, serverOptions),
		mux:      privateMux,
	}

	return server, nil
}

// Identity returns the server's identity.
func (p *Server) Identity() *identity.FullIdentity { return p.tlsOptions.Ident }

// Addr returns the server's public listener address.
func (p *Server) Addr() net.Addr { return p.public.listener.Addr() }

// PrivateAddr returns the server's private listener address.
func (p *Server) PrivateAddr() net.Addr { return p.private.listener.Addr() }

// DRPC returns the server's dRPC mux for registration purposes.
func (p *Server) DRPC() *drpcmux.Mux { return p.public.mux }

// PrivateDRPC returns the server's dRPC mux for registration purposes.
func (p *Server) PrivateDRPC() *drpcmux.Mux { return p.private.mux }

// Close shuts down the server.
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

// Run will run the server and all of its services.
func (p *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

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
	// canceled only after all of the upcoming drpc servers have
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

		return nil
	})

	group.Go(func() error {
		defer cancel()
		return p.public.drpc.Serve(ctx, publicDRPCListener)
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
