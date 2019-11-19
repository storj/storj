// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"sync"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type pendingTransfer struct {
	path             []byte
	pieceSize        int64
	satelliteMessage *pb.SatelliteMessage
	originalPointer  *pb.Pointer
	pieceNum         int32
}

// pendingMap for managing concurrent access to the pending transfer map.
type pendingMap struct {
	mu   sync.RWMutex
	data map[storj.PieceID]*pendingTransfer
}

// newPendingMap creates a new pendingMap and instantiates the map.
func newPendingMap() *pendingMap {
	newData := make(map[storj.PieceID]*pendingTransfer)
	return &pendingMap{
		data: newData,
	}
}

// put adds to the map.
func (pm *pendingMap) put(pieceID storj.PieceID, pendingTransfer *pendingTransfer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.data[pieceID] = pendingTransfer
}

// get returns the pending transfer item from the map, if it exists.
func (pm *pendingMap) get(pieceID storj.PieceID) (pendingTransfer *pendingTransfer, ok bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pendingTransfer, ok = pm.data[pieceID]
	return pendingTransfer, ok
}

// length returns the number of elements in the map.
func (pm *pendingMap) length() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.data)
}

// delete removes the pending transfer item from the map.
func (pm *pendingMap) delete(pieceID storj.PieceID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.data, pieceID)
}
