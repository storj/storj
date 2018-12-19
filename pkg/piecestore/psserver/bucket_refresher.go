// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"flag"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// Error is a standard error class for this package.
var (
	Error                = errs.Class("kademlia bucket refresher error")
	defaultCheckInterval = flag.Duration("piecestore.kbucket-refresher.check-interval", time.Hour, "number of seconds to sleep between updating the kademlia bucket")
)

// refreshService contains the information needed to run the bucket refresher service
type refreshService struct {
	logger *zap.Logger
	rt     dht.RoutingTable
	server *Server
}

func newService(logger *zap.Logger, rt dht.RoutingTable, server *Server) *refreshService {
	return &refreshService{
		logger: logger,
		rt:     rt,
		server: server,
	}
}

// Run runs the bucket refresher service
func (service *refreshService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ticker := time.NewTicker(*defaultCheckInterval)

	for {
		err := service.process(ctx)
		if err != nil {
			service.logger.Error("process", zap.Error(err))
		}

		select {
		case <-ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the bucket refresher service is canceled via context
			return ctx.Err()
		}
	}
}

// process will attempt to update the kademlia bucket with the latest information about the storage node
func (service *refreshService) process(ctx context.Context) error {
	stats, err := service.server.Stats(ctx, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	self := service.rt.Local()

	self.Restrictions = &pb.NodeRestrictions{
		FreeBandwidth: stats.AvailableBandwidth,
		FreeDisk:      stats.AvailableSpace,
	}

	// Update the routing table with latest restrictions
	// TODO (aleitner): Do we want to change the name of ConnectionSuccess?
	if err := service.rt.ConnectionSuccess(&self); err != nil {
		return Error.Wrap(err)
	}

	return nil
}
