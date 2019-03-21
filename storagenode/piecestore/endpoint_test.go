// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/uplink/piecestore"
)

func TestUploadAndPartialDownload(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	expectedData := make([]byte, 100*memory.KiB)
	_, err = rand.Read(expectedData)
	require.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
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

		download, err := planet.Uplinks[0].DownloadStream(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)

		pos, err := download.Seek(tt.offset, io.SeekStart)
		require.NoError(t, err)
		assert.Equal(t, pos, tt.offset)

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
		data := make([]byte, tt.contentLength.Int64())
		_, _ = rand.Read(data[:])

		expectedHash := pkcrypto.SHA256Hash(data)

		var serialNumber storj.SerialNumber
		_, _ = rand.Read(serialNumber[:])

		orderLimit := GenerateOrderLimit(
			t,
			planet.Satellites[0].ID(),
			planet.Uplinks[0].ID(),
			planet.StorageNodes[0].ID(),
			tt.pieceID,
			tt.action,
			serialNumber,
			24*time.Hour,
			24*time.Hour,
			int64(len(data)),
		)
		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
		require.NoError(t, err)

		uploader, err := client.Upload(ctx, orderLimit)
		require.NoError(t, err)

		_, err = uploader.Write(data)
		require.NoError(t, err)

		pieceHash, err := uploader.Commit()
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)

			assert.Equal(t, expectedHash, pieceHash.Hash)

			signee := signing.SignerFromFullIdentity(planet.StorageNodes[0].Identity)
			require.NoError(t, signing.VerifyPieceHashSignature(signee, pieceHash))
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

	var serialNumber storj.SerialNumber
	_, _ = rand.Read(serialNumber[:])

	expectedData := make([]byte, 10*memory.KiB)
	_, _ = rand.Read(expectedData)

	orderLimit := GenerateOrderLimit(
		t,
		planet.Satellites[0].ID(),
		planet.Uplinks[0].ID(),
		planet.StorageNodes[0].ID(),
		storj.PieceID{1},
		pb.PieceAction_PUT,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		int64(len(expectedData)),
	)
	signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
	orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
	require.NoError(t, err)

	uploader, err := client.Upload(ctx, orderLimit)
	require.NoError(t, err)

	_, err = uploader.Write(expectedData)
	require.NoError(t, err)

	_, err = uploader.Commit()
	require.NoError(t, err)

	for _, tt := range []struct {
		pieceID storj.PieceID
		action  pb.PieceAction
		err     string
	}{
		{ // should successfully download data
			pieceID: orderLimit.PieceId,
			action:  pb.PieceAction_GET,
			err:     "",
		},
		{ // should err with piece ID not specified
			pieceID: storj.PieceID{2},
			action:  pb.PieceAction_GET,
			err:     "no such file or directory", // TODO fix returned error
		},
		{ // should successfully download data
			pieceID: orderLimit.PieceId,
			action:  pb.PieceAction_PUT,
			err:     "expected get or get repair or audit action got PUT",
		},
	} {
		var serialNumber storj.SerialNumber
		_, _ = rand.Read(serialNumber[:])

		orderLimit := GenerateOrderLimit(
			t,
			planet.Satellites[0].ID(),
			planet.Uplinks[0].ID(),
			planet.StorageNodes[0].ID(),
			tt.pieceID,
			tt.action,
			serialNumber,
			24*time.Hour,
			24*time.Hour,
			int64(len(expectedData)),
		)
		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
		require.NoError(t, err)

		downloader, err := client.Download(ctx, orderLimit, 0, int64(len(expectedData)))
		require.NoError(t, err)

		buffer := make([]byte, len(expectedData))
		n, err := downloader.Read(buffer)

		if tt.err != "" {
		} else {
			require.NoError(t, err)
			require.Equal(t, expectedData, buffer[:n])
		}

		err = downloader.Close()
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)
		}
	}
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

	var serialNumber storj.SerialNumber
	_, _ = rand.Read(serialNumber[:])

	expectedData := make([]byte, 10*memory.KiB)
	_, _ = rand.Read(expectedData)

	orderLimit := GenerateOrderLimit(
		t,
		planet.Satellites[0].ID(),
		planet.Uplinks[0].ID(),
		planet.StorageNodes[0].ID(),
		storj.PieceID{1},
		pb.PieceAction_PUT,
		serialNumber,
		24*time.Hour,
		24*time.Hour,
		int64(len(expectedData)),
	)
	signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
	orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
	require.NoError(t, err)

	uploader, err := client.Upload(ctx, orderLimit)
	require.NoError(t, err)

	_, err = uploader.Write(expectedData)
	require.NoError(t, err)

	_, err = uploader.Commit()
	require.NoError(t, err)

	for _, tt := range []struct {
		pieceID storj.PieceID
		action  pb.PieceAction
		err     string
	}{
		{ // should successfully download data
			pieceID: orderLimit.PieceId,
			action:  pb.PieceAction_DELETE,
			err:     "",
		},
		{ // should err with piece ID not specified
			pieceID: storj.PieceID{99},
			action:  pb.PieceAction_DELETE,
			err:     "", // TODO should this return error
		},
		{ // should successfully download data
			pieceID: orderLimit.PieceId,
			action:  pb.PieceAction_GET,
			err:     "expected delete action got GET",
		},
	} {
		var serialNumber storj.SerialNumber
		_, _ = rand.Read(serialNumber[:])

		orderLimit := GenerateOrderLimit(
			t,
			planet.Satellites[0].ID(),
			planet.Uplinks[0].ID(),
			planet.StorageNodes[0].ID(),
			tt.pieceID,
			tt.action,
			serialNumber,
			24*time.Hour,
			24*time.Hour,
			100,
		)
		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
		require.NoError(t, err)

		err := client.Delete(ctx, orderLimit)
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)
		}
	}
}

func GenerateOrderLimit(t *testing.T, satellite storj.NodeID, uplink storj.NodeID, storageNode storj.NodeID, pieceID storj.PieceID,
	action pb.PieceAction, serialNumber storj.SerialNumber, pieceExpiration, orderExpiration time.Duration, limit int64) *pb.OrderLimit2 {

	pe, err := ptypes.TimestampProto(time.Now().Add(pieceExpiration))
	require.NoError(t, err)
	oe, err := ptypes.TimestampProto(time.Now().Add(orderExpiration))
	require.NoError(t, err)
	orderLimit := &pb.OrderLimit2{
		SatelliteId:     satellite,
		UplinkId:        uplink,
		StorageNodeId:   storageNode,
		PieceId:         pieceID,
		Action:          action,
		SerialNumber:    serialNumber,
		OrderExpiration: oe,
		PieceExpiration: pe,
		Limit:           limit,
	}
	return orderLimit
}
