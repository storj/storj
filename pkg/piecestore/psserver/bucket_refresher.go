// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
)

var (
	// Error is a standard error class for this package.
	Error = errs.Class("kademlia bucket refresher error")
)

// Monitor contains the information needed to run the bucket refresher service
type Monitor struct { // TODO: rename to something clearer
	log    *zap.Logger
	ticker *time.Ticker
	rt     *kademlia.RoutingTable
	server *Server
}

// NewMonitor creates a disk monitor
func NewMonitor(log *zap.Logger, interval time.Duration, rt *kademlia.RoutingTable, server *Server) *Monitor {
	return &Monitor{
		log:    log,
		ticker: time.NewTicker(interval),
		rt:     rt,
		server: server,
	}
}

// Run runs the bucket refresher service
func (service *Monitor) Run(ctx context.Context) error {
	for {
		err := service.process(ctx)
		if err != nil {
			service.log.Error("process", zap.Error(err))
		}

		select {
		case <-service.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the bucket refresher service is canceled via context
			return ctx.Err()
		}
	}
}

// process will attempt to update the kademlia bucket with the latest information about the storage node
func (service *Monitor) process(ctx context.Context) error {
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
	if err := service.rt.UpdateSelf(&self); err != nil {
		return Error.Wrap(err)
	}

	return nil
}
