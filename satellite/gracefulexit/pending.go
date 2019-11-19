// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"sync"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type PendingTransfer struct {
	path             []byte
	pieceSize        int64
	satelliteMessage *pb.SatelliteMessage
	originalPointer  *pb.Pointer
	pieceNum         int32
}

// PendingMap for managing concurrent access to the pending transfer map.
type PendingMap struct {
	mu   sync.RWMutex
	data map[storj.PieceID]*PendingTransfer
}

// NewPendingMap creates a new PendingMap and instantiates the map.
func NewPendingMap() *PendingMap {
	newData := make(map[storj.PieceID]*PendingTransfer)
	return &PendingMap{
		data: newData,
	}
}

// put adds to the map.
func (pm *PendingMap) Put(pieceID storj.PieceID, PendingTransfer *PendingTransfer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.data[pieceID] = PendingTransfer
}

// get returns the pending transfer item from the map, if it exists.
func (pm *PendingMap) Get(pieceID storj.PieceID) (PendingTransfer *PendingTransfer, ok bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	PendingTransfer, ok = pm.data[pieceID]
	return PendingTransfer, ok
}

// length returns the number of elements in the map.
func (pm *PendingMap) Length() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.data)
}

// delete removes the pending transfer item from the map.
func (pm *PendingMap) Delete(pieceID storj.PieceID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.data[pieceID]; !ok {
		return Error.New("piece ID does not exist in pending map")
	}
	delete(pm.data, pieceID)
	return nil
}

// IsFinished determines whether the work is finished, and blocks if needed.
func (pm *PendingMap) IsFinished(ctx context.Context) (bool, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if len(pm.data) > 0 {
		return false, nil
	}

	// concurrently wait for finish or more work
	// if finish happens first, return true. Otherwise return false.
	return false, err
}

// Finish is called when no more work will be added to the map.
func (pm *PendingMap) Finish() error {
	return nil
}
