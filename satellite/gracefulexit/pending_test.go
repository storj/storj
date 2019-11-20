// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/private/sync2"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite/gracefulexit"
)

func TestPendingBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	newWork := &gracefulexit.PendingTransfer{
		Path:             []byte("testbucket/testfile"),
		PieceSize:        10,
		SatelliteMessage: &pb.SatelliteMessage{},
		OriginalPointer:  &pb.Pointer{},
		PieceNum:         1,
	}

	pieceID := testrand.PieceID()

	pending := gracefulexit.NewPendingMap()

	// put should work
	err := pending.Put(pieceID, newWork)
	require.NoError(t, err)

	// put should return an error if the item already exists
	err = pending.Put(pieceID, newWork)
	require.Error(t, err)

	// get should work
	w, ok := pending.Get(pieceID)
	require.True(t, ok)
	require.True(t, bytes.Equal(newWork.Path, w.Path))

	invalidPieceID := testrand.PieceID()
	_, ok = pending.Get(invalidPieceID)
	require.False(t, ok)

	// IsFinished: there is remaining work to be done -> return false immediately
	finished := pending.IsFinished(ctx)
	require.False(t, finished)

	// finished should work
	err = pending.Finish()
	require.NoError(t, err)

	// finished should error if already called
	err = pending.Finish()
	require.NoError(t, err)

	// should not be allowed to Put new work after finished called
	err = pending.Put(testrand.PieceID(), newWork)
	require.Error(t, err)

	// IsFinished: Finish has been called and there is remaining work -> return false
	finished = pending.IsFinished(ctx)
	require.False(t, finished)

	// delete should work
	err = pending.Delete(pieceID)
	require.NoError(t, err)
	_, ok = pending.Get(pieceID)
	require.False(t, ok)

	// delete should return an error if the item does not exist
	err = pending.Delete(pieceID)
	require.Error(t, err)

	// IsFinished: Finish has been called and there is no remaining work -> return true
	finished = pending.IsFinished(ctx)
	require.True(t, finished)
}

// TestPendingIsFinishedWorkAdded ensures that pending.IsFinished blocks if there is no work, then returns false when new work is added
func TestPendingIsFinishedWorkAdded(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	newWork := &gracefulexit.PendingTransfer{
		Path:             []byte("testbucket/testfile"),
		PieceSize:        10,
		SatelliteMessage: &pb.SatelliteMessage{},
		OriginalPointer:  &pb.Pointer{},
		PieceNum:         1,
	}
	pieceID := testrand.PieceID()
	pending := gracefulexit.NewPendingMap()

	fence := sync2.Fence{}
	var group errgroup.Group
	group.Go(func() error {
		// expect no work
		size := pending.Length()
		require.EqualValues(t, size, 0)

		fence.Release()
		finished := pending.IsFinished(ctx)
		require.False(t, finished)

		// expect new work to be added
		size = pending.Length()
		require.EqualValues(t, size, 1)
		return nil
	})
	group.Go(func() error {
		// wait for IsFinished call to begin before adding work
		require.True(t, fence.Wait(ctx))
		time.Sleep(50 * time.Millisecond)

		err := pending.Put(pieceID, newWork)
		require.NoError(t, err)
		return nil
	})

	require.NoError(t, group.Wait())
}

// TestPendingIsFinishedFinishedCalled ensures that pending.IsFinished blocks if there is no work, then returns true when Finished is called
func TestPendingIsFinishedFinishedCalled(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pending := gracefulexit.NewPendingMap()

	fence := sync2.Fence{}
	var group errgroup.Group
	group.Go(func() error {
		fence.Release()
		finished := pending.IsFinished(ctx)
		require.True(t, finished)
		return nil
	})
	group.Go(func() error {
		// wait for IsFinished call to begin before finishing
		require.True(t, fence.Wait(ctx))
		time.Sleep(50 * time.Millisecond)

		err := pending.Finish()
		require.NoError(t, err)
		return nil
	})

	require.NoError(t, group.Wait())
}

// TestPendingIsFinishedCtxCanceled ensures that pending.IsFinished blocks if there is no work, then returns true when context is canceled
func TestPendingIsFinishedCtxCanceled(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pending := gracefulexit.NewPendingMap()

	ctx2, cancel := context.WithCancel(ctx)
	fence := sync2.Fence{}
	var group errgroup.Group
	group.Go(func() error {
		fence.Release()
		finished := pending.IsFinished(ctx2)
		require.True(t, finished)
		return nil
	})
	group.Go(func() error {
		// wait for IsFinished call to begin before canceling
		require.True(t, fence.Wait(ctx))
		time.Sleep(50 * time.Millisecond)

		cancel()
		return nil
	})

	require.NoError(t, group.Wait())
}
