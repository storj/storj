// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/storagenode/pieces"
)

type Config struct {
	Interval time.Duration
}

// Service which monitors piecestore.Pieces disk usage and updates kademlia
type Service struct {
	log *zap.Logger

	pieces   *pieces.Store
	kademlia *kademlia.Kademlia
}

// TODO: should it be responsible for monitoring actual bandwidth as well?
