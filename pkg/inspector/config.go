// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
	// Error is the main inspector error class for this package
	Error = errs.Class("inspector error:")
)

// Config is passed to CaptPlanet for bootup and configuration
type Config struct {
}

// Run starts up the server and loads configs
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	fmt.Printf("starting up inspector")

	kad := kademlia.LoadFromContext(ctx)
	ol := overlay.LoadFromContext(ctx)

	srv := &Server{
		dht:     kad,
		cache:   ol,
		logger:  zap.L(),
		metrics: monkit.Default,
	}

	fmt.Printf("got to srv %+v\n", srv)

	pb.RegisterInspectorServer(server.GRPC(), srv)

	return server.Run(ctx)
}
