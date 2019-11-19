// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite/gracefulexit"
)

func TestPendingHappyPath(t *testing.T) {
	testcontext.New(t)
	defer testcontext.Cleanup()

	newWork := gracefulexit.PendingTransfer{
		path:             []byte("testbucket/testfile"),
		pieceSize:        10,
		satelliteMessage: &pb.SatelliteMessage{},
		originalPointer:  &pb.Pointer{},
		pieceNum:         1,
	}

	pieceID := testrand.PieceID()

	pending := gracefulexit.NewPendingMap()

	// put should work
	pending.Put(pieceID, pending)

	// get should work
	w, ok := pending.Get(pieceID)
	require.True(t, ok)
	require.Equal(newWork.path, w.path)

	invalidPieceID := testrand.PieceID()
	_, ok = pending.Get(invalidPieceID)
	require.False(t, ok)

	// IsFinished: there is remaining work to be done -> return false immediately
	finished := pending.Isfinished(ctx)
	require.False(t, finished)

	// delete should work
	err := pending.Delete(pieceID)
	require.NoError(t, err)
	_, ok = pending.Get(pieceID)
	require.False(t, ok)

	// delete should return an error if the item does not exist?
	err = pending.Delete(pieceID)
	require.Error(t, err)

	// isFinished cases:
	// DONE there is remaining work to be done -> return false immediately
	// there is no remaining work to be done -> block and return false when new work added
	// there is no remaining work to be done -> block and return true when finish is called
	// there is no remaining work to be done -> block and return when ctx is canceled
	// finish has been called and there is remaining work -> return false
	// finish has been called and there is no remaining work -> return true
}
