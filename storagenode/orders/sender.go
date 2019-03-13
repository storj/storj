// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"time"

	"go.uber.org/zap"

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

type DB interface {
	// Enqueue inserts order to the list of orders needing to be sent to the satellite.
	Enqueue(ctx context.Context, info *Info) error
	// ListUnsent returns orders that haven't been sent yet.
	ListUnsent(ctx context.Context, limit int) ([]*Info, error)
}

type SenderConfig struct {
	Interval time.Duration
}

// Sender which looks through piecestore.Orders and sends them to satellite
// should be roughly copy-paste of agreement sender
type Sender struct {
	log *zap.Logger

	client   transport.Client
	kademlia *kademlia.Kademlia

	table DB

	config SenderConfig
}
