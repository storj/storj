// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package refresher

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("kademlia bucket refresher error")
	mon   = monkit.Package()
)

// refreshService contains the information needed to run the bucket refresher service
type refreshService struct {
	logger *zap.Logger
	ticker *time.Ticker
	rt     *kademlia.RoutingTable
	server *psserver.Server
}

func newService(logger *zap.Logger, interval time.Duration, rt *kademlia.RoutingTable, server *psserver.Server) *refreshService {
	return &refreshService{
		logger: logger,
		ticker: time.NewTicker(interval),
		rt:     rt,
		server: server,
	}
}

// Run runs the bucket refresher service
func (service *refreshService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err := service.process(ctx)
		if err != nil {
			service.logger.Error("process", zap.Error(err))
		}

		select {
		case <-service.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the bucket refresher service is canceled via context
			return ctx.Err()
		}
	}
}

// process will attempt to update the kademlia bucket with the latest information about the storage node
func (service *refreshService) process(ctx context.Context) error {
	stats, err := service.server.Stats(ctx, nil)
	if err != nil {
		return err
	}

	self := service.rt.Local()

	self.Restrictions = &pb.NodeRestrictions{
		FreeBandwidth: stats.AvailableBandwidth,
		FreeDisk:      stats.AvailableSpace,
	}

	// Update the routing table with latest restrictions
	// TODO (aleitner): Do we want to change the name of ConnectionSuccess?
	service.rt.ConnectionSuccess(&self)

	return nil
}
