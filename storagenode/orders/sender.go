// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
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
}

// SenderConfig defines configuration for sending orders.
type SenderConfig struct {
	Interval time.Duration
}

// Sender sends every interval unsent orders to the satellite.
type Sender struct {
	log *zap.Logger

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
		return nil
	})
}

// Close stops the sending service.
func (sender *Sender) Close() error {
	sender.Loop.Stop()
	return nil
}
