// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestDeleter(t *testing.T) {
	cases := []struct {
		testID        string
		deleteToTrash bool
	}{
		{
			testID:        "regular-delete",
			deleteToTrash: false,
		}, {
			testID:        "trash-delete",
			deleteToTrash: true,
		},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
				log := zaptest.NewLogger(t)
				dir, err := filestore.NewDir(log, ctx.Dir("piecedeleter"))
				require.NoError(t, err)

				blobs := filestore.New(log, dir, filestore.DefaultConfig)
				defer ctx.Check(blobs.Close)

				v0PieceInfo, ok := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)
				require.True(t, ok, "V0PieceInfoDB can not satisfy V0PieceInfoDBForTest")

				conf := pieces.Config{
					WritePreallocSize: 4 * memory.MiB,
					DeleteToTrash:     testCase.deleteToTrash,
				}
				store := pieces.NewStore(log, pieces.NewFileWalker(log, blobs, v0PieceInfo), blobs, v0PieceInfo, db.PieceExpirationDB(), nil, conf)
				deleter := pieces.NewDeleter(log, store, 1, 10000)
				defer ctx.Check(deleter.Close)
				deleter.SetupTest()

				require.NoError(t, deleter.Run(ctx))
				satelliteID := testrand.NodeID()
				pieceID := testrand.PieceID()

				data := testrand.Bytes(memory.KB)
				w, err := store.Writer(ctx, satelliteID, pieceID, pb.PieceHashAlgorithm_SHA256)
				require.NoError(t, err)
				_, err = w.Write(data)
				require.NoError(t, err)
				require.NoError(t, w.Commit(ctx, &pb.PieceHeader{}))

				// Delete the piece we've created
				unhandled := deleter.Enqueue(ctx, satelliteID, []pb.PieceID{pieceID})
				require.Equal(t, 0, unhandled)

				// wait for test hook to fire twice
				deleter.Wait(ctx)

				// check that piece is not available
				r1, err := store.Reader(ctx, satelliteID, pieceID)
				require.Error(t, err)
				require.Nil(t, r1)

				defer func() {
					if r1 != nil {
						ctx.Check(r1.Close)
					}
				}()

				// check the trash
				err = store.RestoreTrash(ctx, satelliteID)
				require.NoError(t, err)

				r2, err := store.Reader(ctx, satelliteID, pieceID)
				defer func() {
					if r2 != nil {
						ctx.Check(r2.Close)
					}
				}()
				if !testCase.deleteToTrash {
					require.Error(t, err)
					require.Nil(t, r2)
				}
				if testCase.deleteToTrash {
					require.NoError(t, err)
					require.NotNil(t, r2)
				}

				// Also delete a random non-existent piece, so we know it doesn't blow up when this happens
				unhandled = deleter.Enqueue(ctx, satelliteID, []pb.PieceID{testrand.PieceID()})
				require.Equal(t, 0, unhandled)
			})
		})
	}
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
	}

	for _, tc := range testcases {
		satelliteID := testrand.NodeID()
		pieceIDs := make([]storj.PieceID, 0, tc.pieces)
		for i := 0; i < tc.pieces; i++ {
			pieceIDs = append(pieceIDs, testrand.PieceID())
		}
		deleter := pieces.NewDeleter(zaptest.NewLogger(t), nil, 1, tc.queueSize)
		unhandled := deleter.Enqueue(context.Background(), satelliteID, pieceIDs)
		require.Equal(t, tc.expUnhandled, unhandled)
		require.NoError(t, deleter.Close())
	}
}
