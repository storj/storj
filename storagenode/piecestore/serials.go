// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"sync"

	"github.com/golang/protobuf/ptypes/timestamp"

	"storj.io/storj/pkg/storj"
)

type SerialNumbers struct {
	mu   sync.Mutex
	seen map[SerialNumber]timestamp.Timestamp
}

type SerialNumber struct {
	Satellite    storj.NodeID
	SerialNumber string
}

func LoadSerialNumbers(infos PieceMeta) *SerialNumbers {
	// TODO: load all active them from infos
	panic("TODO")
}

func (serials *SerialNumbers) Add(satelliteID storj.NodeID, serialNumber []byte, expiration *timestamp.Timestamp) bool {
	serials.mu.Lock()
	defer serials.mu.Unlock()

	serial := SerialNumber{satelliteID, string(serialNumber)}
	_, seen := serials.seen[serial]
	if seen {
		return false
	}

	serials.seen[serial] = *expiration
	return true
}

func (serials *SerialNumbers) Collect() {
	panic("TODO")
}
