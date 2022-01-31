// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
	"storj.io/common/rpc/rpctracing"
	"storj.io/drpc/drpcmigrate"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	jaeger "storj.io/monkit-jaeger"
)

// Config holds server specific configuration parameters.
type Config struct {
	tlsopts.Config
	Address        string `user:"true" help:"public address to listen on" default:":7777"`
	PrivateAddress string `user:"true" help:"private address to listen on" default:"127.0.0.1:7778"`
	DisableQUIC    bool   `help:"disable QUIC listener on a server" hidden:"true" default:"false"`

	DisableTCPTLS   bool `help:"disable TCP/TLS listener on a server" internal:"true"`
	DebugLogTraffic bool `hidden:"true" default:"false"` // Deprecated
}

type public struct {
	tcpListener   net.Listener
	udpConn       *net.UDPConn
	quicListener  net.Listener
	addr          net.Addr
	disableTCPTLS bool
	disableQUIC   bool

	drpc *drpcserver.Server
	mux  *drpcmux.Mux
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
func New(log *zap.Logger, tlsOptions *tlsopts.Options, config Config) (_ *Server, err error) {
	server := &Server{
		log:        log,
		tlsOptions: tlsOptions,
		done:       make(chan struct{}),
	}

	server.public, err = newPublic(config.Address, config.DisableTCPTLS, config.DisableQUIC)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serverOptions := drpcserver.Options{
		Manager: rpc.NewDefaultManagerOptions(),
	}
	privateListener, err := net.Listen("tcp", config.PrivateAddress)
	if err != nil {
		return nil, errs.Combine(err, server.public.Close())
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
func (p *Server) Addr() net.Addr { return p.public.addr }

// PrivateAddr returns the server's private listener address.
func (p *Server) PrivateAddr() net.Addr { return p.private.listener.Addr() }

// DRPC returns the server's dRPC mux for registration purposes.
func (p *Server) DRPC() *drpcmux.Mux { return p.public.mux }

// PrivateDRPC returns the server's dRPC mux for registration purposes.
func (p *Server) PrivateDRPC() *drpcmux.Mux { return p.private.mux }

// IsQUICEnabled checks if QUIC is enabled by config and udp port is open.
func (p *Server) IsQUICEnabled() bool { return !p.public.disableQUIC && p.public.udpConn != nil }

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
	_ = p.public.Close()
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

	var (
		publicMux          *drpcmigrate.ListenMux
		publicDRPCListener net.Listener
	)
	if p.public.tcpListener != nil {
		publicMux = drpcmigrate.NewListenMux(p.public.tcpListener, len(drpcmigrate.DRPCHeader))
		publicDRPCListener = tls.NewListener(publicMux.Route(drpcmigrate.DRPCHeader), p.tlsOptions.ServerTLSConfig())
	}

	if p.public.udpConn != nil {
		p.public.quicListener, err = quic.NewListener(p.public.udpConn, p.tlsOptions.ServerTLSConfig(), nil)
		if err != nil {
			return err
		}
	}

	privateMux := drpcmigrate.NewListenMux(p.private.listener, len(drpcmigrate.DRPCHeader))
	privateDRPCListener := privateMux.Route(drpcmigrate.DRPCHeader)

	// We need a new context chain because we require this context to be
	// canceled only after all of the upcoming drpc servers have
	// fully exited. The reason why is because Run closes listener for
	// the mux when it exits, and we can only do that after all of the
	// Servers are no longer accepting.
	muxCtx, muxCancel := context.WithCancel(context.Background())
	defer muxCancel()

	var muxGroup errgroup.Group
	if publicMux != nil {
		muxGroup.Go(func() error {
			return publicMux.Run(muxCtx)
		})
	}
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

	if publicDRPCListener != nil {
		group.Go(func() error {
			defer cancel()
			return p.public.drpc.Serve(ctx, publicDRPCListener)
		})
	}

	if p.public.quicListener != nil {
		group.Go(func() error {
			defer cancel()
			return p.public.drpc.Serve(ctx, wrapListener(p.public.quicListener))
		})
	}

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

func newPublic(publicAddr string, disableTCPTLS, disableQUIC bool) (public, error) {
	var (
		err               error
		publicTCPListener net.Listener
		publicUDPConn     *net.UDPConn
	)

	for retry := 0; ; retry++ {
		addr := publicAddr
		if !disableTCPTLS {
			publicTCPListener, err = net.Listen("tcp", addr)
			if err != nil {
				return public{}, err
			}

			addr = publicTCPListener.Addr().String()
		}

		if !disableQUIC {
			udpAddr, err := net.ResolveUDPAddr("udp", addr)
			if err != nil {
				return public{}, err
			}

			publicUDPConn, err = net.ListenUDP("udp", udpAddr)
			if err != nil {
				_, port, _ := net.SplitHostPort(publicAddr)
				if port == "0" && retry < 10 && isErrorAddressAlreadyInUse(err) {
					// from here, we know for sure that the tcp port chosen by the
					// os is available, but we don't know if the same port number
					// for udp is also available.
					// if a udp port is already in use, we will close the tcp port and retry
					// to find one that is available for both udp and tcp.
					_ = publicTCPListener.Close()
					continue
				}
				return public{}, errs.Combine(err, publicTCPListener.Close())
			}
		}

		break
	}

	publicMux := drpcmux.New()
	publicTracingHandler := rpctracing.NewHandler(publicMux, jaeger.RemoteTraceHandler)
	serverOptions := drpcserver.Options{
		Manager: rpc.NewDefaultManagerOptions(),
	}

	var netAddr net.Addr
	if publicTCPListener != nil {
		netAddr = publicTCPListener.Addr()
	}

	if publicUDPConn != nil && netAddr == nil {
		netAddr = publicUDPConn.LocalAddr()
	}

	return public{
		tcpListener:   wrapListener(publicTCPListener),
		udpConn:       publicUDPConn,
		addr:          netAddr,
		drpc:          drpcserver.NewWithOptions(publicTracingHandler, serverOptions),
		mux:           publicMux,
		disableTCPTLS: disableTCPTLS,
		disableQUIC:   disableQUIC,
	}, nil
}

func (p public) Close() (err error) {
	if p.quicListener != nil {
		err = p.quicListener.Close()
	}
	if p.udpConn != nil {
		err = errs.Combine(err, p.udpConn.Close())
	}
	if p.tcpListener != nil {
		err = errs.Combine(err, p.tcpListener.Close())
	}

	return err
}

// isErrorAddressAlreadyInUse checks whether the error is corresponding to
// EADDRINUSE. Taken from https://stackoverflow.com/a/65865898.
func isErrorAddressAlreadyInUse(err error) bool {
	var eOsSyscall *os.SyscallError
	if !errors.As(err, &eOsSyscall) {
		return false
	}
	var errErrno syscall.Errno
	if !errors.As(eOsSyscall.Err, &errErrno) {
		return false
	}
	if errErrno == syscall.EADDRINUSE {
		return true
	}
	const WSAEADDRINUSE = 10048
	if runtime.GOOS == "windows" && errErrno == WSAEADDRINUSE {
		return true
	}
	return false
}
