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

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

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
}

func TestDownload(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// upload test piece
	client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	expectedData := testrand.Bytes(10 * memory.KiB)
	serialNumber := testrand.SerialNumber()

	orderLimit, piecePrivateKey := GenerateOrderLimit(
		t,
		planet.Satellites[0].ID(),
		planet.StorageNodes[0].ID(),
		storj.PieceID{1},
		pb.PieceAction_PUT,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		int64(len(expectedData)),
	)
	signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
	orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
	require.NoError(t, err)

	uploader, err := client.Upload(ctx, orderLimit, piecePrivateKey)
	require.NoError(t, err)

	_, err = uploader.Write(expectedData)
	require.NoError(t, err)

	_, err = uploader.Commit(ctx)
	require.NoError(t, err)

	for _, tt := range []struct {
		pieceID storj.PieceID
		action  pb.PieceAction
		errs    []string
	}{
		{ // should successfully download data
			pieceID: orderLimit.PieceId,
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
		{ // should successfully download data
			pieceID: orderLimit.PieceId,
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
}

func TestDownloadGetRepair(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// upload test piece
	client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	expectedData := testrand.Bytes(10 * memory.KiB)
	serialNumber := testrand.SerialNumber()

	ulOrderLimit, piecePrivateKey := GenerateOrderLimit(
		t,
		planet.Satellites[0].ID(),
		planet.StorageNodes[0].ID(),
		storj.PieceID{1},
		pb.PieceAction_PUT,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		int64(len(expectedData)),
	)
	signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
	ulOrderLimit, err = signing.SignOrderLimit(ctx, signer, ulOrderLimit)
	require.NoError(t, err)

	uploader, err := client.Upload(ctx, ulOrderLimit, piecePrivateKey)
	require.NoError(t, err)

	_, err = uploader.Write(expectedData)
	require.NoError(t, err)

	originHash, err := uploader.Commit(ctx)
	require.NoError(t, err)

	serialNumber = testrand.SerialNumber()

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
}

func TestDelete(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// upload test piece
	client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	expectedData := testrand.Bytes(10 * memory.KiB)
	serialNumber := testrand.SerialNumber()

	orderLimit, piecePrivateKey := GenerateOrderLimit(
		t,
		planet.Satellites[0].ID(),
		planet.StorageNodes[0].ID(),
		storj.PieceID{1},
		pb.PieceAction_PUT,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		int64(len(expectedData)),
	)
	signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
	orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
	require.NoError(t, err)

	uploader, err := client.Upload(ctx, orderLimit, piecePrivateKey)
	require.NoError(t, err)

	_, err = uploader.Write(expectedData)
	require.NoError(t, err)

	_, err = uploader.Commit(ctx)
	require.NoError(t, err)

	for _, tt := range []struct {
		pieceID storj.PieceID
		action  pb.PieceAction
		err     string
	}{
		{ // should successfully delete data
			pieceID: orderLimit.PieceId,
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
			pieceID: orderLimit.PieceId,
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
