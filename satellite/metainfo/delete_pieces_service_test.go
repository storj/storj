// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storagenode/pieces"
)

func TestNewDeletePiecesService(t *testing.T) {
	type params struct {
		maxConcurrentConns int
		dialer             rpc.Dialer
		log                *zap.Logger
	}
	var testCases = []struct {
		desc   string
		args   params
		errMsg string
	}{
		{
			desc: "ok",
			args: params{
				maxConcurrentConns: 10,
				dialer:             rpc.NewDefaultDialer(nil),
				log:                zaptest.NewLogger(t),
			},
		},
		{
			desc: "error: 0 maxConcurrentCons",
			args: params{
				maxConcurrentConns: 0,
				dialer:             rpc.NewDefaultDialer(nil),
				log:                zaptest.NewLogger(t),
			},
			errMsg: "greater than 0",
		},
		{
			desc: "error: negative maxConcurrentCons",
			args: params{
				maxConcurrentConns: -3,
				dialer:             rpc.NewDefaultDialer(nil),
				log:                zaptest.NewLogger(t),
			},
			errMsg: "greater than 0",
		},
		{
			desc: "error: zero dialer",
			args: params{
				maxConcurrentConns: 87,
				dialer:             rpc.Dialer{},
				log:                zaptest.NewLogger(t),
			},
			errMsg: "dialer cannot be its zero value",
		},
		{
			desc: "error: nil logger",
			args: params{
				maxConcurrentConns: 2,
				dialer:             rpc.NewDefaultDialer(nil),
				log:                nil,
			},
			errMsg: "logger cannot be nil",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			svc, err := metainfo.NewDeletePiecesService(tc.args.log, tc.args.dialer, tc.args.maxConcurrentConns)
			if tc.errMsg != "" {
				require.Error(t, err)
				require.True(t, metainfo.ErrDeletePieces.Has(err), "unexpected error class")
				require.Contains(t, err.Error(), tc.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, svc)

		})
	}
}

func TestDeletePiecesService_DeletePieces(t *testing.T) {
	t.Run("all nodes up", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		var (
			uplnk        = planet.Uplinks[0]
			satelliteSys = planet.Satellites[0]
		)

		{
			data := testrand.Bytes(10 * memory.KiB)
			// Use RSConfig for ensuring that we don't have long-tail cancellations
			// and the upload doesn't leave garbage in the SNs
			err = uplnk.UploadWithClientConfig(ctx, satelliteSys, cmd.Config{
				Client: cmd.ClientConfig{
					SegmentSize: 10 * memory.KiB,
				},
				RS: cmd.RSConfig{
					MinThreshold:     2,
					RepairThreshold:  2,
					SuccessThreshold: 4,
					MaxThreshold:     4,
				},
			},
				"a-bucket", "object-filename", data,
			)
			require.NoError(t, err)
		}

		var (
			totalUsedSpace int64
			nodesPieces    metainfo.NodesPieces
		)
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += usedSpace

			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := metainfo.NodePieces{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			nodesPieces = append(nodesPieces, nodePieces)
		}

		err = satelliteSys.API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 0.75)
		require.NoError(t, err)

		// calculate the SNs used space after delete the pieces
		var totalUsedSpaceAfterDelete int64
		for _, sn := range planet.StorageNodes {
			usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpaceAfterDelete += usedSpace
		}

		// At this point we can only guarantee that the 75% of the SNs pieces
		// are delete due to the success threshold
		deletedUsedSpace := float64(totalUsedSpace-totalUsedSpaceAfterDelete) / float64(totalUsedSpace)
		if deletedUsedSpace < 0.75 {
			t.Fatalf("deleted used space is less than 0.75%%. Got %f", deletedUsedSpace)
		}
	})

	t.Run("some nodes down", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		var (
			uplnk        = planet.Uplinks[0]
			satelliteSys = planet.Satellites[0]
		)

		{
			data := testrand.Bytes(10 * memory.KiB)
			// Use RSConfig for ensuring that we don't have long-tail cancellations
			// and the upload doesn't leave garbage in the SNs
			err = uplnk.UploadWithClientConfig(ctx, satelliteSys, cmd.Config{
				Client: cmd.ClientConfig{
					SegmentSize: 10 * memory.KiB,
				},
				RS: cmd.RSConfig{
					MinThreshold:     2,
					RepairThreshold:  2,
					SuccessThreshold: 4,
					MaxThreshold:     4,
				},
			},
				"a-bucket", "object-filename", data,
			)
			require.NoError(t, err)
		}

		var nodesPieces metainfo.NodesPieces
		for i, sn := range planet.StorageNodes {
			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := metainfo.NodePieces{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			nodesPieces = append(nodesPieces, nodePieces)

			// stop the first 2 SNs before deleting pieces
			if i < 2 {
				require.NoError(t, planet.StopPeer(sn))
			}
		}

		err = satelliteSys.API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 0.9999)
		require.NoError(t, err)

		// Check that storage nodes which are online when deleting pieces don't
		// hold any piece
		var totalUsedSpace int64
		for i := 2; i < len(planet.StorageNodes); i++ {
			usedSpace, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += usedSpace
		}

		require.Zero(t, totalUsedSpace, "totalUsedSpace online nodes")
	})

	t.Run("all nodes down", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		var (
			uplnk        = planet.Uplinks[0]
			satelliteSys = planet.Satellites[0]
		)

		{
			data := testrand.Bytes(10 * memory.KiB)
			// Use RSConfig for ensuring that we don't have long-tail cancellations
			// and the upload doesn't leave garbage in the SNs
			err = uplnk.UploadWithClientConfig(ctx, satelliteSys, cmd.Config{
				Client: cmd.ClientConfig{
					SegmentSize: 10 * memory.KiB,
				},
				RS: cmd.RSConfig{
					MinThreshold:     2,
					RepairThreshold:  2,
					SuccessThreshold: 4,
					MaxThreshold:     4,
				},
			},
				"a-bucket", "object-filename", data,
			)
			require.NoError(t, err)
		}

		var (
			expectedTotalUsedSpace int64
			nodesPieces            metainfo.NodesPieces
		)
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += usedSpace

			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := metainfo.NodePieces{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			nodesPieces = append(nodesPieces, nodePieces)
			require.NoError(t, planet.StopPeer(sn))
		}

		err = satelliteSys.API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 0.9999)
		require.NoError(t, err)

		var totalUsedSpace int64
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += usedSpace
		}

		require.Equal(t, expectedTotalUsedSpace, totalUsedSpace, "totalUsedSpace")
	})

	t.Run("invalid dialer", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		var (
			uplnk        = planet.Uplinks[0]
			satelliteSys = planet.Satellites[0]
		)

		{
			data := testrand.Bytes(10 * memory.KiB)
			// Use RSConfig for ensuring that we don't have long-tail cancellations
			// and the upload doesn't leave garbage in the SNs
			err = uplnk.UploadWithClientConfig(ctx, satelliteSys, cmd.Config{
				Client: cmd.ClientConfig{
					SegmentSize: 10 * memory.KiB,
				},
				RS: cmd.RSConfig{
					MinThreshold:     2,
					RepairThreshold:  2,
					SuccessThreshold: 4,
					MaxThreshold:     4,
				},
			},
				"a-bucket", "object-filename", data,
			)
			require.NoError(t, err)
		}

		var (
			expectedTotalUsedSpace int64
			nodesPieces            metainfo.NodesPieces
		)
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += usedSpace

			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := metainfo.NodePieces{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			nodesPieces = append(nodesPieces, nodePieces)
		}

		// The passed dialer cannot dial nodes because it doesn't have TLSOptions
		dialer := satelliteSys.API.Dialer
		dialer.TLSOptions = nil
		service, err := metainfo.NewDeletePiecesService(
			zaptest.NewLogger(t), dialer, len(nodesPieces)-1,
		)
		require.NoError(t, err)

		err = service.DeletePieces(ctx, nodesPieces, 0.75)
		require.NoError(t, err)

		var totalUsedSpaceAfterDelete int64
		for _, sn := range planet.StorageNodes {
			usedSpace, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpaceAfterDelete += usedSpace
		}

		// because no node can be dialed the SNs used space should be the same
		require.Equal(t, expectedTotalUsedSpace, totalUsedSpaceAfterDelete)
	})

	t.Run("empty nodes pieces", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		err = planet.Satellites[0].API.Metainfo.DeletePiecesService.DeletePieces(ctx, metainfo.NodesPieces{}, 0.75)
		require.Error(t, err)
		assert.False(t, metainfo.ErrDeletePieces.Has(err), "unexpected error class")
		assert.Contains(t, err.Error(), "invalid number of tasks")
	})

	t.Run("invalid threshold", func(t *testing.T) {
		t.Parallel()

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)
		planet.Start(ctx)

		nodesPieces := make(metainfo.NodesPieces, 1)
		nodesPieces[0].Pieces = make([]storj.PieceID, 2)
		err = planet.Satellites[0].API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 1)
		require.Error(t, err)
		assert.False(t, metainfo.ErrDeletePieces.Has(err), "unexpected error class")
		assert.Contains(t, err.Error(), "invalid successThreshold")
	})
}
