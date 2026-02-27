// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
	"storj.io/storj/private/server"
	pb "storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqueue"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Config contains configuration for the jobq server components.
type Config struct {

	// ListenAddress is the address to listen on for incoming connections.
	ListenAddress string `help:"address to listen on" default:":15781" testDefault:"$HOST:0"`

	// InitAlloc is the initial allocation size for the job queue, in bytes.
	InitAlloc memory.Size `help:"initial allocation size for the job queue, in bytes" default:"2GiB"`
	// MaxMemPerPlacement is the maximum memory to be used per placement for
	// storing jobs ready for repair, in bytes.
	MaxMemPerPlacement memory.Size `help:"maximum memory per placement, in bytes" default:"4GiB"`
	// MemReleaseThreshold is the memory release threshold for the job queue, in bytes.
	MemReleaseThreshold memory.Size `help:"element memory release threshold for the job queue, in bytes" default:"100MiB"`
	// RetryAfter is the time to wait before retrying a failed job.
	RetryAfter time.Duration `help:"time to wait before retrying a failed job" default:"1h"`
}

// Module is a mud module that registers jobq server components.
func Module(ball *mud.Ball) {
	mud.Provide[*QueueMap](ball, NewQueueMapFromConfig)
	mud.Provide[*JobqEndpoint](ball, NewEndpoint)

	mud.Provide[*tlsopts.Options](ball, NewTLSOptions)

	config.RegisterConfig[tlsopts.Config](ball, "tls")
	config.RegisterConfig[Config](ball, "server")

	mud.Provide[*server.Server](ball, func(log *zap.Logger, tlsOptions *tlsopts.Options, cfg Config, tlsCfg tlsopts.Config) (*server.Server, error) {
		serverConfig := server.Config{
			Config:      tlsCfg,
			Address:     cfg.ListenAddress,
			DisableQUIC: true,
			TCPFastOpen: false,
		}
		return server.New(log.Named("server"), tlsOptions, serverConfig)
	})

	// endpoint registration - wires the endpoint to the server
	mud.Provide[*EndpointRegistration](ball, func(srv *server.Server, endpoint *JobqEndpoint) (*EndpointRegistration, error) {
		err := pb.DRPCRegisterJobQueue(srv.DRPC(), endpoint)
		if err != nil {
			return nil, err
		}
		return &EndpointRegistration{}, nil
	})
}

// NewQueueMapFromConfig creates a new QueueMap from the given configuration.
func NewQueueMapFromConfig(log *zap.Logger, cfg Config) *QueueMap {
	initElements := uint64(cfg.InitAlloc) / uint64(jobq.RecordSize)
	maxElements := uint64(cfg.MaxMemPerPlacement) / uint64(jobq.RecordSize)
	memReleaseThreshold := uint64(cfg.MemReleaseThreshold) / uint64(jobq.RecordSize)

	log.Debug("initializing job queue",
		zap.Uint64("elements_before_queue_resize", initElements),
		zap.Uint64("element_mem_release_threshold", memReleaseThreshold))

	queueFactory := func(placement storj.PlacementConstraint) (*jobqueue.Queue, error) {
		return jobqueue.NewQueue(log.Named(fmt.Sprintf("placement-%d", placement)), cfg.RetryAfter, int(initElements), int(maxElements), int(memReleaseThreshold))
	}
	return NewQueueMap(log, queueFactory)
}

// EndpointRegistration is a pseudo component to wire server and DRPC endpoints together.
type EndpointRegistration struct{}

// NewTLSOptions creates TLS options for the jobq server with peer CA whitelist applied.
func NewTLSOptions(id *identity.FullIdentity, cfg tlsopts.Config, revocationDB extensions.RevocationDB) (*tlsopts.Options, error) {
	tlsOptions, err := tlsopts.NewOptions(id, cfg, revocationDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS options: %w", err)
	}

	// apply peer CA whitelist
	if err := ApplyPeerCAWhitelist(cfg.UsePeerCAWhitelist, cfg.PeerCAWhitelistPath, tlsOptions); err != nil {
		return nil, fmt.Errorf("failed to apply peer CA whitelist: %w", err)
	}

	return tlsOptions, nil
}

// ApplyPeerCAWhitelist applies the peer CA whitelist to the TLS options.
func ApplyPeerCAWhitelist(usePeerCAWhitelist bool, peerCAWhitelistPath string, tlsOpts *tlsopts.Options) (err error) {
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
