// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

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

func LoadSerialNumbers(infos PieceMeta) (*SerialNumbers, error) {
	// TODO: iterate infos
	return &SerialNumbers{
		seen: map[SerialNumber]time.Time{},
	}, nil
}

func (serials *SerialNumbers) Add(satelliteID storj.NodeID, serialNumber []byte, expiration *timestamp.Timestamp) bool {
	serials.mu.Lock()
	defer serials.mu.Unlock()

	serial := SerialNumber{satelliteID, string(serialNumber)}
	_, seen := serials.seen[serial]
	if seen {
		return false
	}

	// ignoring error, because at this point it should already be verified
	serials.seen[serial], _ = ptypes.Timestamp(expiration)
	return true
}

func (serials *SerialNumbers) Collect() {
	panic("TODO")
}
