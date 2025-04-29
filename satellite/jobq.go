// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime/pprof"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/server"
	pb "storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqueue"
	jobqserver "storj.io/storj/satellite/jobq/server"
)

// JobqConfig is the configuration for the job queue server.
type JobqConfig struct {
	// ListenAddress is the address to listen on for incoming connections.
	ListenAddress string `help:"address to listen on" default:":15781" testDefault:"$HOST:0"`

	// InitAlloc is the initial allocation size for the job queue, in bytes.
	// There is no special need to keep this low; unused parts of the queue
	// allocation will not take up system memory until the queue grows to that
	// size.
	InitAlloc memory.Size `help:"initial allocation size for the job queue, in bytes" default:"2GiB"`
	// MaxMemPerPlacement is the maximum memory to be used per placement for
	// storing jobs ready for repair, in bytes. The queue will not actually
	// consume this amount of memory unless it is full. If full, lower-priority
	// or longer-delayed jobs will be evicted from the queue when new jobs are
	// added.
	MaxMemPerPlacement memory.Size `help:"maximum memory per placement, in bytes" default:"4GiB"`
	// MemReleaseThreshold is the memory release threshold for the job queue, in
	// bytes. When the job queue has more than this amount of memory mapped to
	// empty pages (because the queue shrunk considerably), the unused memory
	// will be marked as unused (if supported) and the OS will be allowed to
	// reclaim it.
	MemReleaseThreshold memory.Size `help:"element memory release threshold for the job queue, in bytes" default:"100MiB"`
	// RetryAfter is the time to wait before retrying a failed job. If jobs are
	// pushed to the queue with a LastAttemptedAt more recent than this duration
	// ago, they will go into the retry queue instead of the repair queue, until
	// they are eligible to go in the repair queue.
	RetryAfter time.Duration `help:"time to wait before retrying a failed job" default:"1h"`
	// TLS is the configuration for the server's TLS.
	TLS tlsopts.Config

	Debug debug.Config
}

// JobqServer represents a running jobq server process, its configuration and
// incidental services.
type JobqServer struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	Config   *JobqConfig

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Jobq struct {
		Server   *server.Server
		QueueMap *jobqserver.QueueMap
		Endpoint *jobqserver.JobqEndpoint
		Listener net.Listener
		TLSOpts  *tlsopts.Options
	}

	Servers  *lifecycle.Group
	Services *lifecycle.Group
}

// NewJobq sets up a new JobqServer.
func NewJobq(log *zap.Logger, identity *identity.FullIdentity, atomicLogLevel *zap.AtomicLevel, config *JobqConfig, revocationDB extensions.RevocationDB) (*JobqServer, error) {
	initElements := uint64(config.InitAlloc) / uint64(jobq.RecordSize)
	maxElements := uint64(config.MaxMemPerPlacement) / uint64(jobq.RecordSize)
	memReleaseThreshold := uint64(config.MemReleaseThreshold) / uint64(jobq.RecordSize)

	peer := &JobqServer{
		Log:      log,
		Identity: identity,
		Config:   config,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup debug
		var err error
		if config.Debug.Addr != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Addr)
			if err != nil {
				withoutStack := errors.New(err.Error())
				peer.Log.Debug("failed to start debug endpoints", zap.Error(withoutStack))
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "API"

		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default,
			debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{ // setup listener
		serverConfig := server.Config{
			Config:      config.TLS,
			Address:     config.ListenAddress,
			DisableQUIC: true,
			TCPFastOpen: false,
		}
		tlsOptions, err := tlsopts.NewOptions(identity, serverConfig.Config, revocationDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS options: %w", err)
		}
		// the tlsopts machinery does not apply the peer CA whitelist to the server
		// side configuration, so we do it here.
		err = applyPeerCAWhitelist(config.TLS.UsePeerCAWhitelist, config.TLS.PeerCAWhitelistPath, tlsOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to apply peer CA whitelist: %w", err)
		}

		srv, err := server.New(log.Named("server"), tlsOptions, serverConfig)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Jobq.Server = srv

		peer.Servers.Add(lifecycle.Item{
			Name:  "server",
			Run:   peer.Jobq.Server.Run,
			Close: peer.Jobq.Server.Close,
		})
	}

	{ // setup endpoint
		log.Debug("initializing job queue", zap.Uint64("elements before queue resize", initElements), zap.Uint64("element mem release threshold", memReleaseThreshold))

		queueFactory := func(placement storj.PlacementConstraint) (*jobqueue.Queue, error) {
			return jobqueue.NewQueue(log.Named(fmt.Sprintf("placement-%d", placement)), config.RetryAfter, int(initElements), int(maxElements), int(memReleaseThreshold))
		}
		peer.Jobq.QueueMap = jobqserver.NewQueueMap(log, queueFactory)
		peer.Jobq.Endpoint = jobqserver.NewEndpoint(log, peer.Jobq.QueueMap)

		if err := pb.DRPCRegisterJobQueue(peer.Jobq.Server.DRPC(), peer.Jobq.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	return peer, nil
}

// Run runs a JobqServer.
func (peer *JobqServer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "jobq-server"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes a JobqServer.
func (peer *JobqServer) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

func applyPeerCAWhitelist(usePeerCAWhitelist bool, peerCAWhitelistPath string, tlsOpts *tlsopts.Options) (err error) {
	if usePeerCAWhitelist {
		whitelist := []byte(tlsopts.DefaultPeerCAWhitelist)
		if peerCAWhitelistPath != "" {
			whitelist, err = os.ReadFile(peerCAWhitelistPath)
			if err != nil {
				return fmt.Errorf("unable to find whitelist file %v: %w", peerCAWhitelistPath, err)
			}
		}
		tlsOpts.PeerCAWhitelist, err = pkcrypto.CertsFromPEM(whitelist)
		if err != nil {
			return err
		}
		tlsOpts.VerificationFuncs.ServerAdd(peertls.VerifyCAWhitelist(tlsOpts.PeerCAWhitelist))
	}
	return nil
}
