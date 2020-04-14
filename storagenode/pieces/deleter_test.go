// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/pieces"
)

func TestDeleter(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dir, err := filestore.NewDir(ctx.Dir("piecedeleter"))
	require.NoError(t, err)

	blobs := filestore.New(zaptest.NewLogger(t), dir, filestore.DefaultConfig)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zaptest.NewLogger(t), blobs, nil, nil, nil, pieces.DefaultConfig)

	// Also test that 0 works for maxWorkers
	deleter := pieces.NewDeleter(zaptest.NewLogger(t), store, 0, 0)
	defer ctx.Check(deleter.Close)
	deleter.SetupTest()

	require.NoError(t, deleter.Run(ctx))

	satelliteID := testrand.NodeID()
	pieceID := testrand.PieceID()

	data := testrand.Bytes(memory.KB)
	w, err := store.Writer(ctx, satelliteID, pieceID)
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Commit(ctx, &pb.PieceHeader{}))

	// confirm we can read the data before delete
	r, err := store.Reader(ctx, satelliteID, pieceID)
	require.NoError(t, err)

	buf, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, data, buf)

	// Delete the piece we've created
	unhandled := deleter.Enqueue(ctx, satelliteID, []pb.PieceID{pieceID})
	require.Equal(t, 0, unhandled)

	// Also delete a random non-existent piece, so we know it doesn't blow up
	// when this happens
	unhandled = deleter.Enqueue(ctx, satelliteID, []pb.PieceID{testrand.PieceID()})
	require.Equal(t, 0, unhandled)

	// wait for test hook to fire twice
	deleter.Wait(ctx)

	_, err = store.Reader(ctx, satelliteID, pieceID)
	require.Condition(t, func() bool {
		return strings.Contains(err.Error(), "file does not exist") ||
			strings.Contains(err.Error(), "The system cannot find the path specified")
	}, "unexpected error message")
}

func TestEnqueueUnhandled(t *testing.T) {
	testcases := []struct {
		queueSize    int
		pieces       int
		expUnhandled int
	}{
		{
			queueSize:    5,
			pieces:       5,
			expUnhandled: 0,
		},
		{
			queueSize:    4,
			pieces:       5,
			expUnhandled: 1,
		},
		{
			queueSize:    1,
			pieces:       10,
			expUnhandled: 9,
		},
		{
			queueSize:    0, // should default to a big number
			pieces:       10,
			expUnhandled: 0,
		},
	}

	for _, tc := range testcases {
		satelliteID := testrand.NodeID()
		pieceIDs := make([]storj.PieceID, 0, tc.pieces)
		for i := 0; i < tc.pieces; i++ {
			pieceIDs = append(pieceIDs, testrand.PieceID())
		}
		deleter := pieces.NewDeleter(zaptest.NewLogger(t), nil, 0, tc.queueSize)
		unhandled := deleter.Enqueue(context.Background(), satelliteID, pieceIDs)
		require.Equal(t, tc.expUnhandled, unhandled)
		require.NoError(t, deleter.Close())
	}
}
