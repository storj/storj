// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"sync"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// PendingFinishedPromise for waiting for information about finished state.
type PendingFinishedPromise struct {
	addedWorkChan      chan struct{}
	finishedCalledChan chan struct{}
}

func newFinishedPromise() *PendingFinishedPromise {
	return &PendingFinishedPromise{
		addedWorkChan:      make(chan struct{}),
		finishedCalledChan: make(chan struct{}),
	}
}

func (promise *PendingFinishedPromise) Wait(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return true, ctx.Err()
	case <-addedWorkChan:
		return false, nil
	case <-finishedCalledChan:
		return true, nil
	}
}

func (promise *PendingFinishedPromise) addedWork() {
	close(promise.addedWorkChan)
}

func (promise *PendingFinishedPromise) finishedCalled() {
	close(promise.finishedCalledChan)
}

type PendingTransfer struct {
	Path             []byte
	PieceSize        int64
	SatelliteMessage *pb.SatelliteMessage
	OriginalPointer  *pb.Pointer
	PieceNum         int32
}

// PendingMap for managing concurrent access to the pending transfer map.
type PendingMap struct {
	mu              sync.RWMutex
	data            map[storj.PieceID]*PendingTransfer
	finished        bool
	finishedPromise *PendingFinishedPromise
}

// NewPendingMap creates a new PendingMap and instantiates the map.
func NewPendingMap() *PendingMap {
	newData := make(map[storj.PieceID]*PendingTransfer)
	return &PendingMap{
		data: newData,
	}
}

// put adds to the map.
func (pm *PendingMap) Put(pieceID storj.PieceID, pendingTransfer *PendingTransfer) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.data[pieceID]; ok {
		return Error.New("piece ID already exists in pending map")
	}

	pm.data[pieceID] = pendingTransfer

	if pm.finishedPromise != nil {
		pm.finishedPromise.addedWork()
		pm.finishedPromise = nil
	}
	return nil
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

// IsFinished returns a promise for the caller to wait on to determine the finished status of the pending map.
func (pm *PendingMap) IsFinishedPromise() PendingFinishedPromise {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.finishedPromise != nil {
		return pm.finishedPromise
	}

	newPromise := newFinishedPromise()

	if len(pm.data) > 0 {
		newPromise.addedWork()
		return newPromise
	}
	if pm.finished {
		newPromise.finishedCalled()
		return newPromise
	}

	pm.IsFinishedPromise = newPromise
	return newPromise
}

// Finish is called when no more work will be added to the map.
func (pm *PendingMap) Finish() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.finishedPromise != nil {
		pm.finishedPromise.finishedCalled()
		pm.finishedPromise = nil
	}
	pm.finished = true
}
