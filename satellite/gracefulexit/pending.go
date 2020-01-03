// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"sync"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// PendingFinishedPromise for waiting for information about finished state.
type PendingFinishedPromise struct {
	addedWorkChan    chan struct{}
	finishCalledChan chan struct{}
	finishErr        error
}

func newFinishedPromise() *PendingFinishedPromise {
	return &PendingFinishedPromise{
		addedWorkChan:    make(chan struct{}),
		finishCalledChan: make(chan struct{}),
	}
}

// Wait should be called (once) after acquiring the finished promise and will return whether the pending map is finished.
func (promise *PendingFinishedPromise) Wait(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return true, ctx.Err()
	case <-promise.addedWorkChan:
		return false, nil
	case <-promise.finishCalledChan:
		return true, promise.finishErr
	}
}

func (promise *PendingFinishedPromise) addedWork() {
	close(promise.addedWorkChan)
}

func (promise *PendingFinishedPromise) finishCalled(err error) {
	promise.finishErr = err
	close(promise.finishCalledChan)
}

// PendingTransfer is the representation of work on the pending map.
// It contains information about a transfer request that has been sent to a storagenode by the satellite.
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
	doneSending     bool
	doneSendingErr  error
	finishedPromise *PendingFinishedPromise
}

// NewPendingMap creates a new PendingMap.
func NewPendingMap() *PendingMap {
	newData := make(map[storj.PieceID]*PendingTransfer)
	return &PendingMap{
		data: newData,
	}
}

// Put adds work to the map. If there is already work associated with this piece ID it returns an error.
// If there is a PendingFinishedPromise waiting, that promise is updated to return false.
func (pm *PendingMap) Put(pieceID storj.PieceID, pendingTransfer *PendingTransfer) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.doneSending {
		return Error.New("cannot add work: pending map already finished")
	}

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

// Get returns the pending transfer item from the map, if it exists.
func (pm *PendingMap) Get(pieceID storj.PieceID) (*PendingTransfer, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pendingTransfer, ok := pm.data[pieceID]
	return pendingTransfer, ok
}

// Length returns the number of elements in the map.
func (pm *PendingMap) Length() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.data)
}

// Delete removes the pending transfer item from the map and returns an error if the data does not exist.
func (pm *PendingMap) Delete(pieceID storj.PieceID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.data[pieceID]; !ok {
		return Error.New("piece ID does not exist in pending map")
	}
	delete(pm.data, pieceID)
	return nil
}

// IsFinishedPromise returns a promise for the caller to wait on to determine the finished status of the pending map.
// If we have enough information to determine the finished status, we update the promise to have an answer immediately.
// Otherwise, we attach the promise to the pending map to be updated and cleared by either Put or DoneSending (whichever happens first).
func (pm *PendingMap) IsFinishedPromise() *PendingFinishedPromise {
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
	if pm.doneSending {
		newPromise.finishCalled(pm.doneSendingErr)
		return newPromise
	}

	pm.finishedPromise = newPromise
	return newPromise
}

// DoneSending is called (with an optional error) when no more work will be added to the map.
// If DoneSending has already been called, an error is returned.
// If a PendingFinishedPromise is waiting on a response, it is updated to return true.
func (pm *PendingMap) DoneSending(err error) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.doneSending {
		return Error.New("DoneSending() already called on pending map")
	}

	if pm.finishedPromise != nil {
		pm.finishedPromise.finishCalled(err)
		pm.finishedPromise = nil
	}
	pm.doneSending = true
	pm.doneSendingErr = err
	return nil
}
