// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"flag"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
)

var (
	// Error defines a Kademlia error
	Error = errs.Class("kademlia error")
	mon   = monkit.Package()
)

var (
	flagBucketSize           = flag.Int("kademlia.bucket-size", 20, "size of each Kademlia bucket")
	flagReplacementCacheSize = flag.Int("kademlia.replacement-cache-size", 5, "size of Kademlia replacement cache")
)

//CtxKey Used as kademlia key
type CtxKey int

const (
	ctxKeyKad CtxKey = iota
)

// OperatorConfig defines properties related to storage node operator metadata
type OperatorConfig struct {
	Email  string `user:"true" help:"operator email address" default:""`
	Wallet string `user:"true" help:"operator wallet adress" default:""`
}

// Config defines all of the things that are needed to start up Kademlia
// server endpoints (and not necessarily client code).
type Config struct {
	BootstrapAddr   string `help:"the Kademlia node to bootstrap against" default:"127.0.0.1:7778"`
	DBPath          string `help:"the path for storage node db services to be created on" default:"$CONFDIR/kademlia"`
	Alpha           int    `help:"alpha is a system wide concurrency parameter" default:"5"`
	ExternalAddress string `user:"true" help:"the public address of the Kademlia node, useful for nodes behind NAT" default:""`
	Operator        OperatorConfig
}

// StorageNodeConfig is a Config that implements provider.Responsibility as
// a storage node
type StorageNodeConfig Config

// Run implements provider.Responsibility
func (c StorageNodeConfig) Run(ctx context.Context, server *provider.Provider) error {
	return Config(c).Run(ctx, server, pb.NodeType_STORAGE)
}

// BootstrapConfig is a Config that implements provider.Responsibility as
// a bootstrap server
type BootstrapConfig Config

// Run implements provider.Responsibility
func (c BootstrapConfig) Run(ctx context.Context, server *provider.Provider) error {
	return Config(c).Run(ctx, server, pb.NodeType_BOOTSTRAP)
}

// SatelliteConfig is a Config that implements provider.Responsibility as
// a satellite
type SatelliteConfig Config

// Run implements provider.Responsibility
func (c SatelliteConfig) Run(ctx context.Context, server *provider.Provider) error {
	return Config(c).Run(ctx, server, pb.NodeType_SATELLITE)
}

// Run does not implement provider.Responsibility. Please use a specific
// SatelliteConfig or StorageNodeConfig
func (c Config) Run(ctx context.Context, server *provider.Provider,
	nodeType pb.NodeType) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO(coyle): I'm thinking we just remove this function and grab from the config.
	in, err := GetIntroNode(c.BootstrapAddr)
	if err != nil {
		return err
	}

	metadata := &pb.NodeMetadata{
		Email:  c.Operator.Email,
		Wallet: c.Operator.Wallet,
	}

	addr := server.Addr().String()
	if c.ExternalAddress != "" {
		addr = c.ExternalAddress
	}

	logger := zap.L()
	kad, err := NewKademlia(logger, nodeType, []pb.Node{*in}, addr, metadata, server.Identity(), c.DBPath, c.Alpha)
	if err != nil {
		return err
	}
	kad.StartRefresh(ctx)
	defer func() { err = utils.CombineErrors(err, kad.Disconnect()) }()

	go func() {
		if err = kad.Bootstrap(ctx); err != nil {
			logger.Error("Failed to bootstrap Kademlia", zap.Any("ID", server.Identity().ID))
		}
	}()

	pb.RegisterNodesServer(server.GRPC(), node.NewServer(logger, kad))

	zap.S().Warn("Once the Peer refactor is done, the kad inspector needs to be registered on a " +
		"gRPC server that only listens on localhost")
	// TODO: register on a private rpc server
	pb.RegisterKadInspectorServer(server.GRPC(), NewInspector(kad, server.Identity()))

	return server.Run(context.WithValue(ctx, ctxKeyKad, kad))
}

// LoadFromContext loads an existing Kademlia from the Provider context
// stack if one exists.
func LoadFromContext(ctx context.Context) *Kademlia {
	if v, ok := ctx.Value(ctxKeyKad).(*Kademlia); ok {
		return v
	}
	return nil
}
