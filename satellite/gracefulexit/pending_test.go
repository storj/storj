// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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
	finishedPromise := pending.IsFinishedPromise()
	finished, err := finishedPromise.Wait(ctx)
	require.False(t, finished)
	require.NoError(t, err)

	// finished should work
	err = pending.DoneSending(nil)
	require.NoError(t, err)

	// finished should error if already called
	err = pending.DoneSending(nil)
	require.Error(t, err)

	// should not be allowed to Put new work after finished called
	err = pending.Put(testrand.PieceID(), newWork)
	require.Error(t, err)

	// IsFinished: Finish has been called and there is remaining work -> return false
	finishedPromise = pending.IsFinishedPromise()
	finished, err = finishedPromise.Wait(ctx)
	require.False(t, finished)
	require.NoError(t, err)

	// delete should work
	err = pending.Delete(pieceID)
	require.NoError(t, err)
	_, ok = pending.Get(pieceID)
	require.False(t, ok)

	// delete should return an error if the item does not exist
	err = pending.Delete(pieceID)
	require.Error(t, err)

	// IsFinished: Finish has been called and there is no remaining work -> return true
	finishedPromise = pending.IsFinishedPromise()
	finished, err = finishedPromise.Wait(ctx)
	require.True(t, finished)
	require.NoError(t, err)
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

		finishedPromise := pending.IsFinishedPromise()

		// wait for work to be added
		fence.Release()

		finished, err := finishedPromise.Wait(ctx)
		require.False(t, finished)
		require.NoError(t, err)

		// expect new work was added
		size = pending.Length()
		require.EqualValues(t, size, 1)
		return nil
	})
	group.Go(func() error {
		// wait for IsFinishedPromise call before adding work
		require.True(t, fence.Wait(ctx))

		err := pending.Put(pieceID, newWork)
		require.NoError(t, err)
		return nil
	})

	require.NoError(t, group.Wait())
}

// TestPendingIsFinishedDoneSendingCalled ensures that pending.IsFinished blocks if there is no work, then returns true when DoneSending is called
func TestPendingIsFinishedDoneSendingCalled(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pending := gracefulexit.NewPendingMap()

	fence := sync2.Fence{}
	var group errgroup.Group
	group.Go(func() error {
		finishedPromise := pending.IsFinishedPromise()

		fence.Release()

		finished, err := finishedPromise.Wait(ctx)
		require.True(t, finished)
		require.NoError(t, err)
		return nil
	})
	group.Go(func() error {
		// wait for IsFinishedPromise call before finishing
		require.True(t, fence.Wait(ctx))

		err := pending.DoneSending(nil)
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
		finishedPromise := pending.IsFinishedPromise()

		fence.Release()

		finished, err := finishedPromise.Wait(ctx2)
		require.True(t, finished)
		require.Error(t, err)
		require.True(t, errs2.IsCanceled(err))
		return nil
	})
	group.Go(func() error {
		// wait for IsFinishedPromise call before canceling
		require.True(t, fence.Wait(ctx))

		cancel()
		return nil
	})

	require.NoError(t, group.Wait())
}

// TestPendingIsFinishedDoneSendingCalledError ensures that pending.IsFinished blocks if there is no work, then returns true with an error when DoneSending is called with an error
func TestPendingIsFinishedDoneSendingCalledError(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pending := gracefulexit.NewPendingMap()

	finishErr := errs.New("test error")
	fence := sync2.Fence{}
	var group errgroup.Group
	group.Go(func() error {
		finishedPromise := pending.IsFinishedPromise()

		fence.Release()

		finished, err := finishedPromise.Wait(ctx)
		require.True(t, finished)
		require.Error(t, err)
		require.Equal(t, finishErr, err)
		return nil
	})
	group.Go(func() error {
		// wait for IsFinishedPromise call before finishing
		require.True(t, fence.Wait(ctx))

		err := pending.DoneSending(finishErr)
		require.NoError(t, err)
		return nil
	})

	require.NoError(t, group.Wait())
}

// TestPendingIsFinishedDoneSendingCalledError2 ensures that pending.IsFinished returns an error if DoneSending was already called with an error.
func TestPendingIsFinishedDoneSendingCalledError2(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	pending := gracefulexit.NewPendingMap()

	finishErr := errs.New("test error")
	err := pending.DoneSending(finishErr)
	require.NoError(t, err)

	finishedPromise := pending.IsFinishedPromise()
	finished, err := finishedPromise.Wait(ctx)
	require.True(t, finished)
	require.Error(t, err)
	require.Equal(t, finishErr, err)
}
