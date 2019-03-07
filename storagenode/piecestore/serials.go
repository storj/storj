// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"sync"
	"time"

	"storj.io/storj/pkg/storj"
)

type SerialNumbers struct {
	mu   sync.Mutex
	seen map[SerialNumber]time.Time
}

type SerialNumber struct {
	Satellite    storj.NodeID
	SerialNumber string
}

func LoadSerialNumbers(infos PieceInfos) *SerialNumbers {
	// TODO: load all active them from infos
	panic("TODO")
}

func (serials *SerialNumber) Add(satelliteID storj.NodeID, serialNumber []byte, expiration time.Time) bool {
	serials.mu.Lock()
	defer serials.mu.Unlock()

	serial := SerialNumber{satelliteID, string(serialNumber)}
	_, seen := serials.seen[serial]
	if seen {
		return false
	}

	serials.seen[serial] = expiration
	return true
}
