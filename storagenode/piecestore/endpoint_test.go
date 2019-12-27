// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/pkcrypto"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/uplink/piecestore"
)

func TestUploadAndPartialDownload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(100 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		assert.NoError(t, err)

		var totalDownload int64
		for _, tt := range []struct {
			offset, size int64
		}{
			{0, 1510},
			{1513, 1584},
			{13581, 4783},
		} {
			if piecestore.DefaultConfig.InitialStep < tt.size {
				t.Fatal("test expects initial step to be larger than size to download")
			}
			totalDownload += piecestore.DefaultConfig.InitialStep

			download, cleanup, err := planet.Uplinks[0].DownloadStreamRange(ctx, planet.Satellites[0], "testbucket", "test/path", tt.offset, -1)
			require.NoError(t, err)
			defer ctx.Check(cleanup)

			data := make([]byte, tt.size)
			n, err := io.ReadFull(download, data)
			require.NoError(t, err)
			assert.Equal(t, int(tt.size), n)

			assert.Equal(t, expectedData[tt.offset:tt.offset+tt.size], data)

			require.NoError(t, download.Close())
		}

		var totalBandwidthUsage bandwidth.Usage
		for _, storagenode := range planet.StorageNodes {
			usage, err := storagenode.DB.Bandwidth().Summary(ctx, time.Now().Add(-10*time.Hour), time.Now().Add(10*time.Hour))
			require.NoError(t, err)
			totalBandwidthUsage.Add(usage)
		}

		err = planet.Uplinks[0].Delete(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)
		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.Error(t, err)

		// check rough limits for the upload and download
		totalUpload := int64(len(expectedData))
		t.Log(totalUpload, totalBandwidthUsage.Put, int64(len(planet.StorageNodes))*totalUpload)
		assert.True(t, totalUpload < totalBandwidthUsage.Put && totalBandwidthUsage.Put < int64(len(planet.StorageNodes))*totalUpload)
		t.Log(totalDownload, totalBandwidthUsage.Get, int64(len(planet.StorageNodes))*totalDownload)
		assert.True(t, totalBandwidthUsage.Get < int64(len(planet.StorageNodes))*totalDownload)
	})
}

func TestUpload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		for _, tt := range []struct {
			pieceID       storj.PieceID
			contentLength memory.Size
			action        pb.PieceAction
			err           string
		}{
			{ // should successfully store data
				pieceID:       storj.PieceID{1},
				contentLength: 50 * memory.KiB,
				action:        pb.PieceAction_PUT,
				err:           "",
			},
			{ // should err with piece ID not specified
				pieceID:       storj.PieceID{},
				contentLength: 1 * memory.KiB,
				action:        pb.PieceAction_PUT,
				err:           "missing piece id",
			},
			{ // should err because invalid action
				pieceID:       storj.PieceID{1},
				contentLength: 1 * memory.KiB,
				action:        pb.PieceAction_GET,
				err:           "expected put or put repair action got GET",
			},
		} {
			data := testrand.Bytes(tt.contentLength)
			serialNumber := testrand.SerialNumber()

			orderLimit, piecePrivateKey := GenerateOrderLimit(
				t,
				planet.Satellites[0].ID(),
				planet.StorageNodes[0].ID(),
				tt.pieceID,
				tt.action,
				serialNumber,
				24*time.Hour,
				24*time.Hour,
				int64(len(data)),
			)
			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
			require.NoError(t, err)

			uploader, err := client.Upload(ctx, orderLimit, piecePrivateKey)
			require.NoError(t, err)

			_, err = uploader.Write(data)
			require.NoError(t, err)

			pieceHash, err := uploader.Commit(ctx)
			if tt.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)

				expectedHash := pkcrypto.SHA256Hash(data)
				assert.Equal(t, expectedHash, pieceHash.Hash)

				signee := signing.SignerFromFullIdentity(planet.StorageNodes[0].Identity)
				require.NoError(t, signing.VerifyPieceHashSignature(ctx, signee, pieceHash))
			}
		}
	})
}

func TestDownload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		pieceID := storj.PieceID{1}
		expectedData, _, _ := uploadPiece(t, ctx, pieceID, planet.StorageNodes[0], planet.Uplinks[0], planet.Satellites[0])
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)

		for _, tt := range []struct {
			pieceID storj.PieceID
			action  pb.PieceAction
			errs    []string
		}{
			{ // should successfully download data
				pieceID: pieceID,
				action:  pb.PieceAction_GET,
			},
			{ // should err with piece ID not specified
				pieceID: storj.PieceID{},
				action:  pb.PieceAction_GET,
				errs:    []string{"missing piece id"},
			},
			{ // should err with piece ID not specified
				pieceID: storj.PieceID{2},
				action:  pb.PieceAction_GET,
				errs:    []string{"file does not exist", "The system cannot find the path specified"},
			},
			{ // should err with invalid action
				pieceID: pieceID,
				action:  pb.PieceAction_PUT,
				errs:    []string{"expected get or get repair or audit action got PUT"},
			},
		} {
			serialNumber := testrand.SerialNumber()

			orderLimit, piecePrivateKey := GenerateOrderLimit(
				t,
				planet.Satellites[0].ID(),
				planet.StorageNodes[0].ID(),
				tt.pieceID,
				tt.action,
				serialNumber,
				24*time.Hour,
				24*time.Hour,
				int64(len(expectedData)),
			)
			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
			require.NoError(t, err)

			downloader, err := client.Download(ctx, orderLimit, piecePrivateKey, 0, int64(len(expectedData)))
			require.NoError(t, err)

			buffer := make([]byte, len(expectedData))
			n, err := downloader.Read(buffer)

			if len(tt.errs) > 0 {
			} else {
				require.NoError(t, err)
				require.Equal(t, expectedData, buffer[:n])
			}

			err = downloader.Close()
			if len(tt.errs) > 0 {
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), tt.errs[0]) || strings.Contains(err.Error(), tt.errs[1]), err.Error())
			} else {
				require.NoError(t, err)
			}

			// these should only be not-nil if action = pb.PieceAction_GET_REPAIR
			hash, originalLimit := downloader.GetHashAndLimit()
			require.Nil(t, hash)
			require.Nil(t, originalLimit)
		}
	})
}

func TestDownloadGetRepair(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		pieceID := storj.PieceID{1}
		expectedData, ulOrderLimit, originHash := uploadPiece(
			t, ctx, pieceID, planet.StorageNodes[0], planet.Uplinks[0], planet.Satellites[0],
		)
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)

		serialNumber := testrand.SerialNumber()

		dlOrderLimit, piecePrivateKey := GenerateOrderLimit(
			t,
			planet.Satellites[0].ID(),
			planet.StorageNodes[0].ID(),
			storj.PieceID{1},
			pb.PieceAction_GET_REPAIR,
			serialNumber,
			24*time.Hour,
			24*time.Hour,
			int64(len(expectedData)),
		)
		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		dlOrderLimit, err = signing.SignOrderLimit(ctx, signer, dlOrderLimit)
		require.NoError(t, err)

		downloader, err := client.Download(ctx, dlOrderLimit, piecePrivateKey, 0, int64(len(expectedData)))
		require.NoError(t, err)

		buffer := make([]byte, len(expectedData))
		n, err := downloader.Read(buffer)

		require.NoError(t, err)
		require.Equal(t, expectedData, buffer[:n])

		err = downloader.Close()
		require.NoError(t, err)

		hash, originLimit := downloader.GetHashAndLimit()
		require.NotNil(t, hash)
		require.Equal(t, originHash.Hash, hash.Hash)
		require.Equal(t, originHash.PieceId, hash.PieceId)

		require.NotNil(t, originLimit)
		require.Equal(t, originLimit.Action, ulOrderLimit.Action)
		require.Equal(t, originLimit.Limit, ulOrderLimit.Limit)
		require.Equal(t, originLimit.PieceId, ulOrderLimit.PieceId)
		require.Equal(t, originLimit.SatelliteId, ulOrderLimit.SatelliteId)
		require.Equal(t, originLimit.SerialNumber, ulOrderLimit.SerialNumber)
		require.Equal(t, originLimit.SatelliteSignature, ulOrderLimit.SatelliteSignature)
		require.Equal(t, originLimit.UplinkPublicKey, ulOrderLimit.UplinkPublicKey)
	})
}

func TestDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		pieceID := storj.PieceID{1}
		uploadPiece(t, ctx, pieceID, planet.StorageNodes[0], planet.Uplinks[0], planet.Satellites[0])
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)

		for _, tt := range []struct {
			pieceID storj.PieceID
			action  pb.PieceAction
			err     string
		}{
			{ // should successfully delete data
				pieceID: pieceID,
				action:  pb.PieceAction_DELETE,
				err:     "",
			},
			{ // should err with piece ID not found
				pieceID: storj.PieceID{99},
				action:  pb.PieceAction_DELETE,
				err:     "", // TODO should this return error
			},
			{ // should err with piece ID not specified
				pieceID: storj.PieceID{},
				action:  pb.PieceAction_DELETE,
				err:     "missing piece id",
			},
			{ // should err due to incorrect action
				pieceID: pieceID,
				action:  pb.PieceAction_GET,
				err:     "expected delete action got GET",
			},
		} {
			serialNumber := testrand.SerialNumber()

			orderLimit, piecePrivateKey := GenerateOrderLimit(
				t,
				planet.Satellites[0].ID(),
				planet.StorageNodes[0].ID(),
				tt.pieceID,
				tt.action,
				serialNumber,
				24*time.Hour,
				24*time.Hour,
				100,
			)
			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
			require.NoError(t, err)

			err := client.Delete(ctx, orderLimit, piecePrivateKey)
			if tt.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)
			}
		}
	})
}

func TestDeletePieces(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var (
			planetSat = planet.Satellites[0]
			planetSN  = planet.StorageNodes[0]
		)

		var client *piecestore.Client
		{
			dossier, err := planetSat.Overlay.DB.Get(ctx.Context, planetSN.ID())
			require.NoError(t, err)

			client, err = piecestore.Dial(
				ctx.Context, planetSat.Dialer, &dossier.Node, zaptest.NewLogger(t), piecestore.Config{},
			)
			require.NoError(t, err)
		}

		t.Run("Ok", func(t *testing.T) {
			pieceIDs := []storj.PieceID{{1}, {2}, {3}, {4}}
			dataArray := make([][]byte, len(pieceIDs))
			for i, pieceID := range pieceIDs {
				dataArray[i], _, _ = uploadPiece(t, ctx, pieceID, planetSN, planet.Uplinks[0], planetSat)
			}

			err := client.DeletePieces(ctx.Context, pieceIDs...)
			require.NoError(t, err)

			for i, pieceID := range pieceIDs {
				_, err = downloadPiece(t, ctx, pieceID, int64(len(dataArray[i])), planetSN, planet.Uplinks[0], planetSat)
				require.Error(t, err)
			}
			require.Condition(t, func() bool {
				return strings.Contains(err.Error(), "file does not exist") ||
					strings.Contains(err.Error(), "The system cannot find the path specified")
			}, "unexpected error message")
		})

		t.Run("Ok: one piece to delete is missing", func(t *testing.T) {
			missingPieceID := storj.PieceID{12}
			pieceIDs := []storj.PieceID{{1}, {2}, {3}, {4}}
			dataArray := make([][]byte, len(pieceIDs))
			for i, pieceID := range pieceIDs {
				dataArray[i], _, _ = uploadPiece(t, ctx, pieceID, planetSN, planet.Uplinks[0], planetSat)
			}

			err := client.DeletePieces(ctx.Context, append(pieceIDs, missingPieceID)...)
			require.NoError(t, err)

			for i, pieceID := range pieceIDs {
				_, err = downloadPiece(t, ctx, pieceID, int64(len(dataArray[i])), planetSN, planet.Uplinks[0], planetSat)
				require.Error(t, err)
			}
			require.Condition(t, func() bool {
				return strings.Contains(err.Error(), "file does not exist") ||
					strings.Contains(err.Error(), "The system cannot find the path specified")
			}, "unexpected error message")
		})

		t.Run("Ok: no piece deleted", func(t *testing.T) {
			pieceID := storj.PieceID{10}
			data, _, _ := uploadPiece(t, ctx, pieceID, planetSN, planet.Uplinks[0], planetSat)

			err := client.DeletePieces(ctx.Context)
			require.NoError(t, err)

			downloaded, err := downloadPiece(t, ctx, pieceID, int64(len(data)), planetSN, planet.Uplinks[0], planetSat)
			require.NoError(t, err)
			require.Equal(t, data, downloaded)
		})

		t.Run("error: permission denied", func(t *testing.T) {
			pieceID := storj.PieceID{11}
			data, _, _ := uploadPiece(t, ctx, pieceID, planetSN, planet.Uplinks[0], planetSat)

			client, err := planet.Uplinks[0].DialPiecestore(ctx, planetSN)
			require.NoError(t, err)

			err = client.DeletePieces(ctx.Context, pieceID)
			require.Error(t, err)
			require.Equal(t, rpcstatus.PermissionDenied, rpcstatus.Code(err))

			downloaded, err := downloadPiece(t, ctx, pieceID, int64(len(data)), planetSN, planet.Uplinks[0], planetSat)
			require.NoError(t, err)
			require.Equal(t, data, downloaded)
		})
	})
}
func TestDeletePiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var (
			planetSat = planet.Satellites[0]
			planetSN  = planet.StorageNodes[0]
		)

		var client *piecestore.Client
		{
			dossier, err := planetSat.Overlay.DB.Get(ctx.Context, planetSN.ID())
			require.NoError(t, err)

			client, err = piecestore.Dial(
				ctx.Context, planetSat.Dialer, &dossier.Node, zaptest.NewLogger(t), piecestore.Config{},
			)
			require.NoError(t, err)
		}

		t.Run("Ok", func(t *testing.T) {
			pieceID := storj.PieceID{1}
			data, _, _ := uploadPiece(t, ctx, pieceID, planetSN, planet.Uplinks[0], planetSat)

			err := client.DeletePiece(ctx.Context, pieceID)
			require.NoError(t, err)

			_, err = downloadPiece(t, ctx, pieceID, int64(len(data)), planetSN, planet.Uplinks[0], planetSat)
			require.Error(t, err)

			require.Condition(t, func() bool {
				return strings.Contains(err.Error(), "file does not exist") ||
					strings.Contains(err.Error(), "The system cannot find the path specified")
			}, "unexpected error message")
		})

		t.Run("error: Not found", func(t *testing.T) {
			err := client.DeletePiece(ctx.Context, storj.PieceID{2})
			require.Error(t, err)
			require.Equal(t, rpcstatus.NotFound, rpcstatus.Code(err))
		})

		t.Run("error: permission denied", func(t *testing.T) {
			client, err := planet.Uplinks[0].DialPiecestore(ctx, planetSN)
			require.NoError(t, err)

			err = client.DeletePiece(ctx.Context, storj.PieceID{})
			require.Error(t, err)
			require.Equal(t, rpcstatus.PermissionDenied, rpcstatus.Code(err))
		})
	})
}

func TestTooManyRequests(t *testing.T) {
	t.Skip("flaky, because of EOF issues")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const uplinkCount = 6
	const maxConcurrent = 3
	const expectedFailures = uplinkCount - maxConcurrent

	log := zaptest.NewLogger(t)

	planet, err := testplanet.NewCustom(log, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: uplinkCount,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Storage2.MaxConcurrentRequests = maxConcurrent
			},
		},
	})
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	doneWaiting := make(chan struct{})
	failedCount := int64(expectedFailures)

	uploads, _ := errgroup.WithContext(ctx)
	defer ctx.Check(uploads.Wait)

	for i, uplink := range planet.Uplinks {
		i, uplink := i, uplink
		uploads.Go(func() (err error) {
			storageNode := planet.StorageNodes[0].Local()
			config := piecestore.DefaultConfig
			config.UploadBufferSize = 0 // disable buffering so we can detect write error early

			client, err := piecestore.Dial(ctx, uplink.Dialer, &storageNode.Node, uplink.Log, config)
			if err != nil {
				return err
			}
			defer func() {
				if cerr := client.Close(); cerr != nil {
					uplink.Log.Error("close failed", zap.Error(cerr))
					err = errs.Combine(err, cerr)
				}
			}()

			pieceID := storj.PieceID{byte(i + 1)}
			serialNumber := testrand.SerialNumber()

			orderLimit, piecePrivateKey := GenerateOrderLimit(
				t,
				planet.Satellites[0].ID(),
				planet.StorageNodes[0].ID(),
				pieceID,
				pb.PieceAction_PUT,
				serialNumber,
				24*time.Hour,
				24*time.Hour,
				int64(10000),
			)

			satelliteSigner := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			orderLimit, err = signing.SignOrderLimit(ctx, satelliteSigner, orderLimit)
			if err != nil {
				return err
			}

			upload, err := client.Upload(ctx, orderLimit, piecePrivateKey)
			if err != nil {
				if errs2.IsRPC(err, rpcstatus.Unavailable) {
					if atomic.AddInt64(&failedCount, -1) == 0 {
						close(doneWaiting)
					}
					return nil
				}
				uplink.Log.Error("upload failed", zap.Stringer("Piece ID", pieceID), zap.Error(err))
				return err
			}

			_, err = upload.Write(make([]byte, orderLimit.Limit))
			if err != nil {
				if errs2.IsRPC(err, rpcstatus.Unavailable) {
					if atomic.AddInt64(&failedCount, -1) == 0 {
						close(doneWaiting)
					}
					return nil
				}
				uplink.Log.Error("write failed", zap.Stringer("Piece ID", pieceID), zap.Error(err))
				return err
			}

			_, err = upload.Commit(ctx)
			if err != nil {
				if errs2.IsRPC(err, rpcstatus.Unavailable) {
					if atomic.AddInt64(&failedCount, -1) == 0 {
						close(doneWaiting)
					}
					return nil
				}
				uplink.Log.Error("commit failed", zap.Stringer("Piece ID", pieceID), zap.Error(err))
				return err
			}

			return nil
		})
	}
}

func GenerateOrderLimit(t *testing.T, satellite storj.NodeID, storageNode storj.NodeID, pieceID storj.PieceID, action pb.PieceAction, serialNumber storj.SerialNumber, pieceExpiration, orderExpiration time.Duration, limit int64) (*pb.OrderLimit, storj.PiecePrivateKey) {
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	now := time.Now()
	return &pb.OrderLimit{
		SatelliteId:     satellite,
		UplinkPublicKey: piecePublicKey,
		StorageNodeId:   storageNode,
		PieceId:         pieceID,
		Action:          action,
		SerialNumber:    serialNumber,
		OrderCreation:   time.Now(),
		OrderExpiration: now.Add(orderExpiration),
		PieceExpiration: now.Add(pieceExpiration),
		Limit:           limit,
	}, piecePrivateKey
}

// uploadPiece uploads piece to storageNode.
func uploadPiece(
	t *testing.T, ctx *testcontext.Context, piece storj.PieceID, storageNode *storagenode.Peer,
	uplink *testplanet.Uplink, satellite *testplanet.SatelliteSystem,
) (uploadedData []byte, _ *pb.OrderLimit, _ *pb.PieceHash) {
	t.Helper()

	client, err := uplink.DialPiecestore(ctx, storageNode)
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	serialNumber := testrand.SerialNumber()
	uploadedData = testrand.Bytes(10 * memory.KiB)

	orderLimit, piecePrivateKey := GenerateOrderLimit(
		t,
		satellite.ID(),
		storageNode.ID(),
		piece,
		pb.PieceAction_PUT,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		int64(len(uploadedData)),
	)
	signer := signing.SignerFromFullIdentity(satellite.Identity)
	orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
	require.NoError(t, err)

	uploader, err := client.Upload(ctx, orderLimit, piecePrivateKey)
	require.NoError(t, err)

	_, err = uploader.Write(uploadedData)
	require.NoError(t, err)

	hash, err := uploader.Commit(ctx)
	require.NoError(t, err)

	return uploadedData, orderLimit, hash
}

// downloadPiece downlodads piece from storageNode.
func downloadPiece(
	t *testing.T, ctx *testcontext.Context, piece storj.PieceID, limit int64,
	storageNode *storagenode.Peer, uplink *testplanet.Uplink, satellite *testplanet.SatelliteSystem,
) (pieceData []byte, err error) {
	t.Helper()

	serialNumber := testrand.SerialNumber()
	orderLimit, piecePrivateKey := GenerateOrderLimit(
		t,
		satellite.ID(),
		storageNode.ID(),
		piece,
		pb.PieceAction_GET,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		limit,
	)
	signer := signing.SignerFromFullIdentity(satellite.Identity)
	orderLimit, err = signing.SignOrderLimit(ctx.Context, signer, orderLimit)
	require.NoError(t, err)

	client, err := uplink.DialPiecestore(ctx, storageNode)
	require.NoError(t, err)

	downloader, err := client.Download(ctx.Context, orderLimit, piecePrivateKey, 0, limit)
	require.NoError(t, err)
	defer func() {
		if err != nil {
			// Ignore err in Close if an error happened in Download because it's also
			// returned by Close.
			_ = downloader.Close()
			return
		}

		err = downloader.Close()
	}()

	buffer := make([]byte, limit)
	n, err := downloader.Read(buffer)
	return buffer[:n], err
}
