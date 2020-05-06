// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testblobs"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
)

func TestService_New_Error(t *testing.T) {
	log := zaptest.NewLogger(t)
	dialer := rpc.NewDefaultDialer(nil)

	_, err := piecedeletion.NewService(nil, dialer, piecedeletion.Config{
		MaxConcurrency:      8,
		MaxPiecesPerBatch:   0,
		MaxPiecesPerRequest: 0,
		DialTimeout:         time.Second,
		FailThreshold:       5 * time.Minute,
	})
	require.True(t, piecedeletion.Error.Has(err), err)
	require.Contains(t, err.Error(), "log is nil")

	_, err = piecedeletion.NewService(log, rpc.Dialer{}, piecedeletion.Config{
		MaxConcurrency: 87,
		DialTimeout:    time.Second,
	})
	//require.True(t, metainfo.ErrDeletePieces.Has(err), err)
	require.Contains(t, err.Error(), "dialer is zero")

	_, err = piecedeletion.NewService(log, dialer, piecedeletion.Config{
		MaxConcurrency: 0,
		DialTimeout:    time.Second,
	})
	require.True(t, piecedeletion.Error.Has(err), err)
	require.Contains(t, err.Error(), "greater than 0")

	_, err = piecedeletion.NewService(log, dialer, piecedeletion.Config{
		MaxConcurrency: -3,
		DialTimeout:    time.Second,
	})
	require.True(t, piecedeletion.Error.Has(err), err)
	require.Contains(t, err.Error(), "greater than 0")

	_, err = piecedeletion.NewService(log, dialer, piecedeletion.Config{
		MaxConcurrency: 3,
		DialTimeout:    time.Nanosecond,
	})
	require.True(t, piecedeletion.Error.Has(err), err)
	require.Contains(t, err.Error(), "dial timeout 1ns must be between 5ms and 5m0s")

	_, err = piecedeletion.NewService(log, dialer, piecedeletion.Config{
		MaxConcurrency: 3,
		DialTimeout:    time.Hour,
	})
	require.True(t, piecedeletion.Error.Has(err), err)
	require.Contains(t, err.Error(), "dial timeout 1h0m0s must be between 5ms and 5m0s")
}

func TestService_DeletePieces_AllNodesUp(t *testing.T) {
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

		percentExp := 0.75

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

		// ensure that no requests return an error
		err := satelliteSys.API.Metainfo.PieceDeletion.Delete(ctx, nil, percentExp)
		require.NoError(t, err)

		var (
			totalUsedSpace int64
			requests       []piecedeletion.Request
		)
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += piecesTotal

			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := piecedeletion.Request{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			requests = append(requests, nodePieces)
		}

		err = satelliteSys.API.Metainfo.PieceDeletion.Delete(ctx, requests, percentExp)
		require.NoError(t, err)

		planet.WaitForStorageNodeDeleters(ctx)

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
		if deletedUsedSpace < percentExp {
			t.Fatalf("deleted used space is less than %e%%. Got %f", percentExp, deletedUsedSpace)
		}
	})
}

func TestService_DeletePieces_SomeNodesDown(t *testing.T) {
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
		numToShutdown := 2

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

		var requests []piecedeletion.Request

		for i, sn := range planet.StorageNodes {
			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := piecedeletion.Request{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			requests = append(requests, nodePieces)

			// stop the first numToShutdown SNs before deleting pieces
			if i < numToShutdown {
				require.NoError(t, planet.StopPeer(sn))
			}
		}

		err := satelliteSys.API.Metainfo.PieceDeletion.Delete(ctx, requests, 0.9999)
		require.NoError(t, err)

		planet.WaitForStorageNodeDeleters(ctx)

		// Check that storage nodes which are online when deleting pieces don't
		// hold any piece
		var totalUsedSpace int64
		for i := numToShutdown; i < len(planet.StorageNodes); i++ {
			piecesTotal, _, err := planet.StorageNodes[i].Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			totalUsedSpace += piecesTotal
		}

		require.Zero(t, totalUsedSpace, "totalUsedSpace online nodes")
	})
}

func TestService_DeletePieces_AllNodesDown(t *testing.T) {
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
			requests               []piecedeletion.Request
		)
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += piecesTotal

			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := piecedeletion.Request{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			requests = append(requests, nodePieces)
			require.NoError(t, planet.StopPeer(sn))
		}

		err := satelliteSys.API.Metainfo.PieceDeletion.Delete(ctx, requests, 0.9999)
		require.NoError(t, err)

		planet.WaitForStorageNodeDeleters(ctx)

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

func TestService_DeletePieces_Invalid(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].API.Metainfo.PieceDeletion

		nodesPieces := []piecedeletion.Request{
			{Pieces: make([]storj.PieceID, 1)},
			{Pieces: make([]storj.PieceID, 1)},
		}
		err := service.Delete(ctx, nodesPieces, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request #0 is invalid")
	})
}

func TestService_DeletePieces_Timeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.PieceDeletion.RequestTimeout = 200 * time.Millisecond
				config.Metainfo.RS.MinThreshold = 2
				config.Metainfo.RS.RepairThreshold = 2
				config.Metainfo.RS.SuccessThreshold = 4
				config.Metainfo.RS.TotalThreshold = 4
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
			requests               []piecedeletion.Request
		)
		for _, sn := range planet.StorageNodes {
			// calculate the SNs total used space after data upload
			piecesTotal, _, err := sn.Storage2.Store.SpaceUsedForPieces(ctx)
			require.NoError(t, err)
			expectedTotalUsedSpace += piecesTotal

			// Get pb node and all the pieces of the storage node
			dossier, err := satelliteSys.Overlay.Service.Get(ctx, sn.ID())
			require.NoError(t, err)

			nodePieces := piecedeletion.Request{
				Node: &dossier.Node,
			}

			err = sn.Storage2.Store.WalkSatellitePieces(ctx, satelliteSys.ID(),
				func(store pieces.StoredPieceAccess) error {
					nodePieces.Pieces = append(nodePieces.Pieces, store.PieceID())
					return nil
				},
			)
			require.NoError(t, err)

			requests = append(requests, nodePieces)

			// make delete operation on storage nodes slow
			storageNodeDB := sn.DB.(*testblobs.SlowDB)
			delay := 500 * time.Millisecond
			storageNodeDB.SetLatency(delay)
		}

		err := satelliteSys.API.Metainfo.PieceDeletion.Delete(ctx, requests, 0.75)
		require.NoError(t, err)
		// A timeout error won't be propagated up to the service level
		// but we'll know that the deletes didn't happen based on usedSpace
		// check below.

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
