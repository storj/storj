// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"sync"
	"time"

	"storj.io/storj/pkg/storj"
)

type Trust interface {
	IsTrustedSatellite(storj.NodeID) error
	VerifySatelliteSignature([]byte, storj.NodeID) error
}

type serialNumber struct {
	Satellite storj.NodeID
	Piece     storj.PieceID2
}

type serialNumbers struct {
	mu   sync.Mutex
	seen map[serialNumber]time.Time
}

func (serials *serialNumbers) Add(satelliteID storj.NodeID, pieceID storj.PieceID2, expiration time.Time) bool {
	serials.mu.Lock()
	defer serials.mu.Unlock()

	serial := serialNumber{satelliteID, pieceID}
	_, seen := serials.seen[serial]
	if seen {
		return false
	}
	serials.seen[serial] = expiration
	return true
}

type Endpoint struct {
	seenSerialNumbers serialNumbers
}

func (endpoint *Endpoint) Delete(delete pb.PieceDeleteRequest) {
	err := endpoint.verifyOrderLimit(delete.Limit)
}
