// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/storagenode/pieces"
)

// Config defines parameters for storage node disk and bandwidth usage monitoring.
type Config struct {
	Interval time.Duration
}

// Service which monitors disk usage and updates kademlia network as necessary.
type Service struct {
	log      *zap.Logger
	pieces   *pieces.Store
	kademlia *kademlia.Kademlia
}

// NewService creates a new storage node monitoring service.
func NewService(log *zap.Logger, pieces *pieces.Store, kademlia *kademlia.Kademlia) *Service {
	return &Service{
		log:      log,
		pieces:   pieces,
		kademlia: kademlia,
	}
}

// TODO: should it be responsible for monitoring actual bandwidth as well?
