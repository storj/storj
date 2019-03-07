// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
)

type Config struct {
	Interval time.Duration
}

// Service which looks through piecestore.PieceInfos and deletes them from piecestore.Pieces
// should roughly correspond to pkg/piecestore/psserver.Collector from previous.
type Service struct {
	log *zap.Logger

	pieces *pieces.Store
	meta   piecestore.PieceMeta
}
