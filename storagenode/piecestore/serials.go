// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"storj.io/storj/pkg/storj"
)

// SerialNumberFn is callback from IterateAll
type SerialNumberFn func(satelliteID storj.NodeID, serialNumber []byte, expiration time.Time)

// UsedSerials is a persistent store for serial numbers.
// TODO: maybe this should be in orders.UsedSerials
type UsedSerials interface {
	// Add adds a serial to the database.
	Add(ctx context.Context, satelliteID storj.NodeID, serialNumber []byte, expiration time.Time) error
	// DeleteExpired deletes expired serial numbers
	DeleteExpired(ctx context.Context, now time.Time) error

	// IterateAll iterates all serials.
	// Note, this will lock the database and should only be used during startup.
	IterateAll(ctx context.Context, fn SerialNumberFn) error
}

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
