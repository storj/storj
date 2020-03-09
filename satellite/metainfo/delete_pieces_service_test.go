// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/memory"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testblobs"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
)

func TestDeletePiecesService_New_Error(t *testing.T) {
	log := zaptest.NewLogger(t)
	dialer := rpc.NewDefaultDialer(nil)

	_, err := metainfo.NewDeletePiecesService(nil, dialer, metainfo.DeletePiecesServiceConfig{
		MaxConcurrentConnection: 8,
		NodeOperationTimeout:    time.Second,
	})
	require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "logger cannot be nil")

	_, err = metainfo.NewDeletePiecesService(log, rpc.Dialer{}, metainfo.DeletePiecesServiceConfig{
		MaxConcurrentConnection: 87,
		NodeOperationTimeout:    time.Second,
	})
	require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "dialer cannot be its zero value")

	_, err = metainfo.NewDeletePiecesService(log, dialer, metainfo.DeletePiecesServiceConfig{
		MaxConcurrentConnection: 0,
		NodeOperationTimeout:    time.Second,
	})
	require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "greater than 0")

	_, err = metainfo.NewDeletePiecesService(log, dialer, metainfo.DeletePiecesServiceConfig{
		MaxConcurrentConnection: -3,
		NodeOperationTimeout:    time.Second,
	})
	require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "greater than 0")

	_, err = metainfo.NewDeletePiecesService(log, dialer, metainfo.DeletePiecesServiceConfig{
		MaxConcurrentConnection: 3,
		NodeOperationTimeout:    time.Nanosecond,
	})
	require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "node operation timeout")

	_, err = metainfo.NewDeletePiecesService(log, dialer, metainfo.DeletePiecesServiceConfig{
		MaxConcurrentConnection: 3,
		NodeOperationTimeout:    time.Hour,
	})
	require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "node operation timeout")
}

func TestDeletePiecesService_DeletePieces_AllNodesUp(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		// Use RSConfig for ensuring that we don't have long-tail cancellations
		// and the upload doesn't leave garbage in the SNs
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplnk := planet.Uplinks[0]
		satelliteSys := planet.Satellites[0]

		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
				Client: testplanet.ClientConfig{
					SegmentSize: 10 * memory.KiB,
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
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += piecesTotal

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

		err := satelliteSys.API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 0.75)
		require.NoError(t, err)

		// calculate the SNs used space after delete the pieces
		var totalUsedSpaceAfterDelete int64
		for _, sn := range planet.StorageNodes {
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpaceAfterDelete += piecesTotal
		}

		// At this point we can only guarantee that the 75% of the SNs pieces
		// are delete due to the success threshold
		deletedUsedSpace := float64(totalUsedSpace-totalUsedSpaceAfterDelete) / float64(totalUsedSpace)
		if deletedUsedSpace < 0.75 {
			t.Fatalf("deleted used space is less than 0.75%%. Got %f", deletedUsedSpace)
		}
	})
}

func TestDeletePiecesService_DeletePieces_SomeNodesDown(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		// Use RSConfig for ensuring that we don't have long-tail cancellations
		// and the upload doesn't leave garbage in the SNs
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplnk := planet.Uplinks[0]
		satelliteSys := planet.Satellites[0]

		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
				Client: testplanet.ClientConfig{
					SegmentSize: 10 * memory.KiB,
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

		err := satelliteSys.API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 0.9999)
		require.NoError(t, err)

		// Check that storage nodes which are online when deleting pieces don't
		// hold any piece
		var totalUsedSpace int64
		for i := 2; i < len(planet.StorageNodes); i++ {
			piecesTotal, _, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += piecesTotal
		}

		require.Zero(t, totalUsedSpace, "totalUsedSpace online nodes")
	})
}

func TestDeletePiecesService_DeletePieces_AllNodesDown(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		// Use RSConfig for ensuring that we don't have long-tail cancellations
		// and the upload doesn't leave garbage in the SNs
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplnk := planet.Uplinks[0]
		satelliteSys := planet.Satellites[0]

		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
				Client: testplanet.ClientConfig{
					SegmentSize: 10 * memory.KiB,
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
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += piecesTotal

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

		err := satelliteSys.API.Metainfo.DeletePiecesService.DeletePieces(ctx, nodesPieces, 0.9999)
		require.NoError(t, err)

		var totalUsedSpace int64
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += piecesTotal
		}

		require.Equal(t, expectedTotalUsedSpace, totalUsedSpace, "totalUsedSpace")
	})
}

func TestDeletePiecesService_DeletePieces_InvalidDialer(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplnk := planet.Uplinks[0]
		satelliteSys := planet.Satellites[0]

		{
			data := testrand.Bytes(10 * memory.KiB)
			// Use RSConfig for ensuring that we don't have long-tail cancellations
			// and the upload doesn't leave garbage in the SNs
			err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
				Client: testplanet.ClientConfig{
					SegmentSize: 10 * memory.KiB,
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
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += piecesTotal

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
			zaptest.NewLogger(t), dialer, metainfo.DeletePiecesServiceConfig{
				MaxConcurrentConnection: len(nodesPieces) - 1,
				NodeOperationTimeout:    2 * time.Second,
			},
		)
		require.NoError(t, err)
		defer ctx.Check(service.Close)

		err = service.DeletePieces(ctx, nodesPieces, 0.75)
		require.NoError(t, err)

		var totalUsedSpaceAfterDelete int64
		for _, sn := range planet.StorageNodes {
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpaceAfterDelete += piecesTotal
		}

		// because no node can be dialed the SNs used space should be the same
		require.Equal(t, expectedTotalUsedSpace, totalUsedSpaceAfterDelete)
	})
}

func TestDeletePiecesService_DeletePieces_Invalid(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		deletePiecesService := planet.Satellites[0].API.Metainfo.DeletePiecesService

		t.Run("empty node pieces", func(t *testing.T) {
			t.Parallel()
			err := deletePiecesService.DeletePieces(ctx, metainfo.NodesPieces{}, 0.75)
			require.Error(t, err)
			assert.False(t, metainfo.ErrDeletePieces.Has(err), err)
			assert.Contains(t, err.Error(), "invalid number of tasks")
		})

		t.Run("invalid threshold", func(t *testing.T) {
			t.Parallel()
			nodesPieces := metainfo.NodesPieces{
				{Pieces: make([]storj.PieceID, 1)},
				{Pieces: make([]storj.PieceID, 1)},
			}
			err := deletePiecesService.DeletePieces(ctx, nodesPieces, 1)
			require.Error(t, err)
			assert.False(t, metainfo.ErrDeletePieces.Has(err), err)
			assert.Contains(t, err.Error(), "invalid successThreshold")
		})
	})
}

func TestDeletePiecesService_DeletePieces_Timeout(t *testing.T) {
	deletePiecesServiceConfig := metainfo.DeletePiecesServiceConfig{}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.DeletePiecesService.NodeOperationTimeout = 200 * time.Millisecond
				config.Metainfo.RS.MinThreshold = 2
				config.Metainfo.RS.RepairThreshold = 2
				config.Metainfo.RS.SuccessThreshold = 4
				config.Metainfo.RS.TotalThreshold = 4
				deletePiecesServiceConfig = config.Metainfo.DeletePiecesService
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplnk := planet.Uplinks[0]
		satelliteSys := planet.Satellites[0]

		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.UploadWithClientConfig(ctx, satelliteSys, testplanet.UplinkConfig{
				Client: testplanet.ClientConfig{
					SegmentSize: 10 * memory.KiB,
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
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += piecesTotal

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

			// make delete operation on storage nodes slow
			storageNodeDB := sn.DB.(*testblobs.SlowDB)
			delay := 200 * time.Millisecond
			storageNodeDB.SetLatency(delay)
		}

		core, recorded := observer.New(zapcore.DebugLevel)
		log := zap.New(core)
		service, err := metainfo.NewDeletePiecesService(log, satelliteSys.Dialer, deletePiecesServiceConfig)
		require.NoError(t, err)

		err = service.DeletePieces(ctx, nodesPieces, 0.75)
		require.NoError(t, err)

		// get all delete failure logs
		logEntries := recorded.FilterMessageSnippet("unable to delete pieces")
		require.Equal(t, logEntries.Len(), 4)

		// error messages from the logs should all contain context canceled error
		for _, entry := range logEntries.All() {
			field := entry.ContextMap()
			errMsg, ok := field["error"]
			require.True(t, ok)
			require.Contains(t, errMsg, "context deadline exceeded")
		}

		var totalUsedSpace int64
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += piecesTotal
		}

		require.Equal(t, expectedTotalUsedSpace, totalUsedSpace, "totalUsedSpace")
	})
}
