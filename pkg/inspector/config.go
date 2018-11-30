// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
)

var (
	mon = monkit.Package()
	// Error is the main inspector error class for this package
	Error = errs.Class("inspector server error:")
)

// Config is passed to CaptPlanet for bootup and configuration
type Config struct {
	Enabled bool `help:"enable or disable the inspector" default:"true"`
}

// Run starts up the server and loads configs
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	kad := kademlia.LoadFromContext(ctx)
	if kad == nil {
		return Error.New("programmer error: kademlia responsibility unstarted")
	}

	ol := overlay.LoadFromContext(ctx)
	if ol == nil {
		return Error.New("programmer error: overlay responsibility unstarted")
	}

	sdb := statdb.LoadFromContext(ctx)
	if sdb == nil {
		return Error.New("programmer error: statdb responsibility unstarted")
	}

	id, err := provider.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		return Error.New("error creating inspector identity:")
	}

	srv := &Server{
		dht:      kad,
		identity: id,
		cache:    ol,
		statdb:   sdb,
		logger:   zap.L(),
		metrics:  monkit.Default,
	}

	pb.RegisterInspectorServer(server.GRPC(), srv)

	return server.Run(ctx)
}
