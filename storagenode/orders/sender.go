// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

type Table interface {
	// not sure whether there's a need to duplicate, but just in case
	Add(ctx context.Context, limit *pb.OrderLimit2, order *pb.Order2) error
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

	table Table

	config SenderConfig
}
