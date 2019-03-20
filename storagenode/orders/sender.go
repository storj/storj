// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// Info contains full information about an order.
type Info struct {
	Limit  *pb.OrderLimit2
	Order  *pb.Order2
	Uplink *identity.PeerIdentity
}

// DB implements storing orders for sending to the satellite.
type DB interface {
	// Enqueue inserts order to the list of orders needing to be sent to the satellite.
	Enqueue(ctx context.Context, info *Info) error
	// ListUnsent returns orders that haven't been sent yet.
	ListUnsent(ctx context.Context, limit int) ([]*Info, error)
	// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
	ListUnsentBySatellite(ctx context.Context) (map[storj.NodeID]*Info, error)
	// Archive marks order as being handled.
	Archive(ctx context.Context, satellite storj.NodeID, serial storj.SerialNumber) error
}

// SenderConfig defines configuration for sending orders.
type SenderConfig struct {
	Interval      time.Duration
	SettleTimeout time.Duration
}

// Sender sends every interval unsent orders to the satellite.
type Sender struct {
	log    *zap.Logger
	config SenderConfig

	client   transport.Client
	kademlia *kademlia.Kademlia
	orders   DB

	Loop sync2.Cycle
}

// NewSender creates an order sender.
func NewSender(log *zap.Logger, client transport.Client, kademlia *kademlia.Kademlia, orders DB, config SenderConfig) *Sender {
	return &Sender{
		log:      log,
		client:   client,
		kademlia: kademlia,
		orders:   orders,

		Loop: *sync2.NewCycle(config.Interval),
	}
}

// Run sends orders on every interval to the appropriate satellites.
func (sender *Sender) Run(ctx context.Context) error {
	return sender.Loop.Run(ctx, func(ctx context.Context) error {
		sender.log.Debug("sending")

		ordersBySatellite, err := sender.orders.ListUnsentBySatellite(ctx)
		if err != nil {
			sender.log.Error("listing orders", zap.Error(err))
			return nil
		}

		if len(ordersBySatellite) > 0 {
			var group errgroup.Group
			ctx, cancel := context.WithTimeout(ctx, sender.config.SettleTimeout)

			for satellite, agreements := range ordersBySatellite {
				group.Go(func() error {
				})
			}
		}

		return nil
	})
}

// Settle uploads agreements to the satellite.
func (sender *Sender) Settle(ctx context.Context, satellite storj.NodeID, orders []*Info) {

}

// Close stops the sending service.
func (sender *Sender) Close() error {
	sender.Loop.Stop()
	return nil
}
