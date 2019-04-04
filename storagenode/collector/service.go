// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/storagenode/pieces"
)

// Config defines parameters for storage node Collector.
type Config struct {
	Interval time.Duration
}

// Service implements collecting expired pieces on the storage node.
type Service struct {
	log        *zap.Logger
	pieces     *pieces.Store
	pieceinfos pieces.DB
}

// NewService creates a new collector service.
func NewService(log *zap.Logger, pieces *pieces.Store, pieceinfos pieces.DB) *Service {
	return &Service{
		log:        log,
		pieces:     pieces,
		pieceinfos: pieceinfos,
	}
}
