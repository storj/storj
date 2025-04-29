// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/jtolio/noiseconn"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/experiment"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/noise"
	"storj.io/common/rpc/quic"
	"storj.io/common/rpc/rpctracing"
	"storj.io/drpc"
	"storj.io/drpc/drpcmigrate"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/drpc/drpcstats"
	jaeger "storj.io/monkit-jaeger"
	"storj.io/storj/private/server/debounce"
)

const (
	// tcpMaxPacketAge is the maximum amount of time we expect to worry about
	// an undelivered TCP packet lingering in the network. TCP TTL isn't
	// supposed to exceed about 4 minutes, so this is double that with
	// padding.
	tcpMaxPacketAge = 10 * time.Minute
	// debounceLimit is the amount of times the server should worry about
	// debouncing incoming noise  or TLS messages, per message. debouncing
	// won't happen if the number of identical packets received is larger than
	// this.
	debounceLimit = 2
)

// Config holds server specific configuration parameters.
type Config struct {
	tlsopts.Config
	Address        string `user:"true" help:"public address to listen on" default:":7777" testDefault:"$HOST:0"`
	PrivateAddress string `user:"true" help:"private address to listen on" default:"127.0.0.1:7778" testDefault:""`
	DisableQUIC    bool   `help:"disable QUIC listener on a server" hidden:"true" default:"false"`

	DisableTCP      bool `help:"disable TCP listener on a server" internal:"true"`
	DebugLogTraffic bool `hidden:"true" default:"false"` // Deprecated

	TCPFastOpen       bool `help:"enable support for tcp fast open" default:"true"`
	TCPFastOpenQueue  int  `help:"the size of the tcp fast open queue" default:"256"`
	DebouncingEnabled bool `help:"whether to debounce incoming messages" default:"true"`
}

// Server represents a bundle of services defined by a specific ID.
// Examples of servers are the satellite, the storagenode, and the uplink.
type Server struct {
	log        *zap.Logger
	tlsOptions *tlsopts.Options
	noiseConf  noise.Config
	config     Config
	fastOpen   bool

	publicTCPListener  net.Listener
	publicUDPConn      *net.UDPConn
	publicQUICListener net.Listener
	privateTCPListener net.Listener
	addr               net.Addr

	publicEndpointsReplaySafe *endpointCollection
	publicEndpointsAll        *endpointCollection
	privateEndpoints          *endpointCollection

	// http fallback for the public endpoint
	publicHTTP http.HandlerFunc

	mu   sync.Mutex
	wg   sync.WaitGroup
	once sync.Once
	done chan struct{}
}

// New creates a Server out of an Identity, a net.Listener,
// and interceptors.
func New(log *zap.Logger, tlsOptions *tlsopts.Options, config Config) (_ *Server, err error) {
	noiseConf, err := noise.GenerateServerConf(noise.DefaultProto, tlsOptions.Ident)
	if err != nil {
		return nil, err
	}

	server := &Server{
		log:        log,
		tlsOptions: tlsOptions,
		noiseConf:  noiseConf,
		config:     config,

		publicEndpointsReplaySafe: newEndpointCollection(),
		publicEndpointsAll:        newEndpointCollection(),
		privateEndpoints:          newEndpointCollection(),

		done: make(chan struct{}),
	}

	listenConfig := net.ListenConfig{}
	if config.TCPFastOpen {
		server.fastOpen = tryInitFastOpen(log)
		if server.fastOpen {
			listenConfig.Control = func(network, address string, c syscall.RawConn) error {
				return c.Control(func(fd uintptr) {
					err := setTCPFastOpen(fd, config.TCPFastOpenQueue)
					if err != nil {
						log.Sugar().Infof("failed to set tcp fast open for this socket: %v", err)
					}
				})
			}
		}
	}

	for retry := 0; ; retry++ {
		addr := config.Address
		if !config.DisableTCP {
			publicTCPListener, err := listenConfig.Listen(context.Background(), "tcp", addr)
			if err != nil {
				return nil, err
			}
			addr = publicTCPListener.Addr().String()
			server.publicTCPListener = wrapListener(publicTCPListener)
		}

		if !config.DisableQUIC {
			udpAddr, err := net.ResolveUDPAddr("udp", addr)
			if err != nil {
				if server.publicTCPListener != nil {
					_ = server.publicTCPListener.Close()
				}
				return nil, err
			}

			publicUDPConn, err := net.ListenUDP("udp", udpAddr)
			if err != nil {
				_, port, splitErr := net.SplitHostPort(config.Address)
				if splitErr == nil && port == "0" && retry < 10 && isErrorAddressAlreadyInUse(err) {
					// from here, we know for sure that the tcp port chosen by the
					// os is available, but we don't know if the same port number
					// for udp is also available.
					// if a udp port is already in use, we will close the tcp port and retry
					// to find one that is available for both udp and tcp.
					if server.publicTCPListener != nil {
						_ = server.publicTCPListener.Close()
					}
					continue
				}
				if server.publicTCPListener != nil {
					return nil, errs.Combine(err, server.publicTCPListener.Close())
				}
				return nil, err
			}
			server.publicUDPConn = publicUDPConn
		}

		break
	}

	if server.publicTCPListener != nil {
		server.addr = server.publicTCPListener.Addr()
	} else if server.publicUDPConn != nil {
		server.addr = server.publicUDPConn.LocalAddr()
	}

	if config.PrivateAddress != "" {
		privateTCPListener, err := net.Listen("tcp", config.PrivateAddress)
		if err != nil {
			return nil, errs.Combine(err, server.Close())
		}
		server.privateTCPListener = wrapListener(privateTCPListener)
	}

	return server, nil
}

// Stats implements monkit.StatSource and outputs statistics about drpc bytes read
// and written per rpc.
func (p *Server) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	merge := func(out, in map[string]drpcstats.Stats) {
		for k, vi := range in {
			vo := out[k]
			vo.Read += vi.Read
			vo.Written += vi.Written
			out[k] = vo
		}
	}

	stats := make(map[string]drpcstats.Stats)
	merge(stats, p.publicEndpointsReplaySafe.drpc.Stats())
	merge(stats, p.publicEndpointsAll.drpc.Stats())
	merge(stats, p.privateEndpoints.drpc.Stats())

	rpcs := make([]string, 0, len(stats))
	for k := range stats {
		rpcs = append(rpcs, k)
	}
	sort.Strings(rpcs)

	for _, rpc := range rpcs {
		v := stats[rpc]
		cb(monkit.NewSeriesKey("drpc_bytes_transmitted").WithTag("rpc", rpc), "read", float64(v.Read))
		cb(monkit.NewSeriesKey("drpc_bytes_transmitted").WithTag("rpc", rpc), "written", float64(v.Written))
	}
}

// Identity returns the server's identity.
func (p *Server) Identity() *identity.FullIdentity { return p.tlsOptions.Ident }

// Addr returns the server's public listener address.
func (p *Server) Addr() net.Addr { return p.addr }

// PrivateAddr returns the server's private listener address.
func (p *Server) PrivateAddr() net.Addr { return p.privateTCPListener.Addr() }

// DRPC returns the server's DRPC mux that supports all endpoints for
// registration purposes.
func (p *Server) DRPC() drpc.Mux {
	return p.publicEndpointsAll.mux
}

// ReplaySafeDRPC returns the server's DRPC mux that supports replay safe
// endpoints for registration purposes.
func (p *Server) ReplaySafeDRPC() drpc.Mux {
	return p.publicEndpointsReplaySafe.mux
}

// PrivateDRPC returns the server's DRPC mux for registration purposes.
func (p *Server) PrivateDRPC() drpc.Mux { return p.privateEndpoints.mux }

// IsQUICEnabled checks if QUIC is enabled by config and udp port is open.
func (p *Server) IsQUICEnabled() bool { return !p.config.DisableQUIC && p.publicUDPConn != nil }

// NoiseKeyAttestation returns the noise key attestation for this server.
func (p *Server) NoiseKeyAttestation(ctx context.Context) (_ *pb.NoiseKeyAttestation, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := noise.ConfigToInfo(p.noiseConf)
	if err != nil {
		return nil, err
	}
	return noise.GenerateKeyAttestation(ctx, p.tlsOptions.Ident, info)
}

// DebounceLimit is the amount of times the server is able to
// debounce incoming noise or TLS messages, per message.
func (p *Server) DebounceLimit() int {
	if !p.config.DebouncingEnabled {
		return 0
	}
	return debounceLimit
}

// FastOpen returns true if FastOpen is possibly open. false means we
// know FastOpen is off.
func (p *Server) FastOpen() bool {
	return p.fastOpen
}

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
	if p.publicQUICListener != nil {
		_ = p.publicQUICListener.Close()
	}
	if p.publicUDPConn != nil {
		_ = p.publicUDPConn.Close()
	}
	if p.publicTCPListener != nil {
		_ = p.publicTCPListener.Close()
	}
	if p.privateTCPListener != nil {
		_ = p.privateTCPListener.Close()
	}
	return nil
}

// AddHTTPFallback adds http fallback to the drpc endpoint.
func (p *Server) AddHTTPFallback(httpHandler http.HandlerFunc) {
	p.publicHTTP = httpHandler
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
		publicTLSDRPCListener   net.Listener
		publicDefaultListener   net.Listener
		publicNoiseDRPCListener net.Listener
		publicHTTPListener      net.Listener
		privateDRPCListener     net.Listener
		privateDefaultListener  net.Listener
	)

	if p.publicUDPConn != nil {
		// TODO: we goofed here. we need something like a drpcmigrate.ListenMux
		// for UDP packets.
		publicQUICListener, err := quic.NewListener(p.publicUDPConn, p.tlsOptions.ServerTLSConfig(), nil)
		if err != nil {
			return err
		}
		// TODO: this is also strange. why does (*Server).Close() need to close
		// the quic listener? Shouldn't closing p.publicUDPConn be enough?
		// We should be able to remove UDP-specific protocols from the Server
		// struct and keep them localized to (*Server).Run, akin to TLS vs
		// Noise drpcmigrate.ListenMuxed listeners over TCP.
		p.publicQUICListener = wrapListener(publicQUICListener)
	}

	// We need a new context chain because we require this context to be
	// canceled only after all of the upcoming drpc servers have
	// fully exited. The reason why is because Run closes listener for
	// the mux when it exits, and we can only do that after all of the
	// Servers are no longer accepting.
	muxCtx, muxCancel := context.WithCancel(context.Background())
	defer muxCancel()

	var muxGroup errgroup.Group

	if p.publicTCPListener != nil {
		publicLMux := drpcmigrate.NewListenMux(p.publicTCPListener, len(drpcmigrate.DRPCHeader))
		tlsMux := publicLMux.Route(drpcmigrate.DRPCHeader)
		publicDefaultListener = publicLMux.Default()

		var noiseOpts noiseconn.Options

		if p.config.DebouncingEnabled {
			debouncer := debounce.NewDebouncer(tcpMaxPacketAge, debounceLimit)
			tlsMux = tlsDebounce(tlsMux, debouncer.ResponderFirstMessageValidator)
			noiseOpts.ResponderFirstMessageValidator = debouncer.ResponderFirstMessageValidator
		}

		publicTLSDRPCListener = tls.NewListener(tlsMux, p.tlsOptions.ServerTLSConfig())
		publicNoiseDRPCListener = noiseconn.NewListenerWithOptions(
			publicLMux.Route(noise.Header),
			p.noiseConf,
			noiseOpts)
		if p.publicHTTP != nil {
			publicHTTPListener = NewPrefixedListener([]byte("GET / HT"), publicLMux.Route("GET / HT"))
		}
		muxGroup.Go(func() error {
			return publicLMux.Run(muxCtx)
		})
	}

	if p.privateTCPListener != nil {
		privateLMux := drpcmigrate.NewListenMux(p.privateTCPListener, len(drpcmigrate.DRPCHeader))
		privateDRPCListener = privateLMux.Route(drpcmigrate.DRPCHeader)
		privateDefaultListener = privateLMux.Default()

		muxGroup.Go(func() error {
			return privateLMux.Run(muxCtx)
		})
	}

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

	connectListenerToEndpoints := func(ctx context.Context, listener net.Listener, endpoints *endpointCollection) {
		if listener != nil {
			group.Go(func() error {
				defer cancel()
				return endpoints.drpc.Serve(ctx, listener)
			})
		}
	}

	connectListenerToEndpoints(ctx, publicTLSDRPCListener, p.publicEndpointsAll)
	connectListenerToEndpoints(ctx, p.publicQUICListener, p.publicEndpointsAll)
	connectListenerToEndpoints(ctx, publicNoiseDRPCListener, p.publicEndpointsReplaySafe)
	connectListenerToEndpoints(ctx, privateDRPCListener, p.privateEndpoints)

	rejectConnections := func(ctx context.Context, listener net.Listener) {
		if listener == nil {
			return
		}
		group.Go(func() error {
			defer cancel()

			for {
				conn, err := listener.Accept()
				if err != nil {
					return nil //nolint: nilerr // error is ignored, as no action is required. Probably just closing the listener.
				}
				_ = conn.Close()
			}

		})

		// shutdown the previous listener loop, if context is cancelled
		group.Go(func() error {
			<-ctx.Done()

			_ = listener.Close()
			return nil
		})
	}

	// we can reject connections without valid mux prefix
	rejectConnections(ctx, publicDefaultListener)
	rejectConnections(ctx, privateDefaultListener)

	if publicHTTPListener != nil {
		// this http server listens on the filtered messages of the incoming DRPC port, instead of a separated port
		httpServer := http.Server{
			Handler: p.publicHTTP,
		}

		group.Go(func() error {
			<-ctx.Done()
			return httpServer.Shutdown(context.Background())
		})
		group.Go(func() error {
			defer cancel()
			err := httpServer.Serve(publicHTTPListener)
			if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			return err
		})
	}

	// Now we wait for all the stuff using the listeners to exit.
	err = group.Wait()

	// Now we close down our listeners.
	muxCancel()
	return errs.Combine(err, muxGroup.Wait())
}

type endpointCollection struct {
	mux  *drpcmux.Mux
	drpc *drpcserver.Server
}

func newEndpointCollection() *endpointCollection {
	mux := drpcmux.New()
	return &endpointCollection{
		mux: mux,
		drpc: drpcserver.NewWithOptions(
			experiment.NewHandler(
				rpctracing.NewHandler(
					mux,
					jaeger.RemoteTraceHandler),
			),
			drpcserver.Options{
				Manager:      rpc.NewDefaultManagerOptions(),
				CollectStats: true,
			},
		),
	}
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
