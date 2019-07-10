// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

const oneWeek = 7 * 24 * time.Hour

func TestOrderLimitPutValidation(t *testing.T) {
	for i, tt := range []struct {
		useUnknownSatellite bool
		pieceID             storj.PieceID
		action              pb.PieceAction
		serialNumber        storj.SerialNumber
		pieceExpiration     time.Duration
		orderExpiration     time.Duration
		limit               int64
		availableBandwidth  int64
		availableSpace      int64
		err                 string
	}{
		{ // unapproved satellite id
			useUnknownSatellite: true,
			pieceID:             storj.PieceID{1},
			action:              pb.PieceAction_PUT,
			serialNumber:        storj.SerialNumber{1},
			pieceExpiration:     oneWeek,
			orderExpiration:     oneWeek,
			limit:               memory.KiB.Int64(),
			err:                 " is untrusted",
		},
		{ // approved satellite id
			pieceID:         storj.PieceID{2},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{2},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           10 * memory.KiB.Int64(),
		},
		{ // wrong action type
			pieceID:         storj.PieceID{3},
			action:          pb.PieceAction_GET,
			serialNumber:    storj.SerialNumber{3},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           memory.KiB.Int64(),
			err:             "expected put or put repair action got GET",
		},
		{ // piece expired
			pieceID:         storj.PieceID{4},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{4},
			pieceExpiration: -4 * 24 * time.Hour,
			orderExpiration: oneWeek,
			limit:           memory.KiB.Int64(),
			err:             "piece expired:",
		},
		{ // limit is negative
			pieceID:         storj.PieceID{5},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{5},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           -1,
			err:             "order limit is negative",
		},
		{ // order limit expired
			pieceID:         storj.PieceID{6},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{6},
			pieceExpiration: oneWeek,
			orderExpiration: -4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
			err:             "order expired:",
		},
		{ // allocated bandwidth limit
			pieceID:            storj.PieceID{7},
			action:             pb.PieceAction_PUT,
			serialNumber:       storj.SerialNumber{7},
			pieceExpiration:    oneWeek,
			orderExpiration:    oneWeek,
			limit:              10 * memory.KiB.Int64(),
			availableBandwidth: 5 * memory.KiB.Int64(),
			err:                "out of bandwidth",
		},
		{ // allocated space limit
			pieceID:         storj.PieceID{8},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{8},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           10 * memory.KiB.Int64(),
			availableSpace:  5 * memory.KiB.Int64(),
			err:             "out of space",
		},
	} {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 1, 1)
		require.NoError(t, err)
		defer ctx.Check(planet.Shutdown)

		planet.Start(ctx)

		// set desirable bandwidth
		setBandwidth(ctx, t, planet, tt.availableBandwidth)
		// set desirable space
		setSpace(ctx, t, planet, tt.availableSpace)

		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		satellite := planet.Satellites[0].Identity
		if tt.useUnknownSatellite {
			unapprovedSatellite, err := planet.NewIdentity()
			require.NoError(t, err)
			signer = signing.SignerFromFullIdentity(unapprovedSatellite)
			satellite = unapprovedSatellite
		}

		orderLimit := GenerateOrderLimit(
			t,
			satellite.ID,
			planet.Uplinks[0].ID(),
			planet.StorageNodes[0].ID(),
			tt.pieceID,
			tt.action,
			tt.serialNumber,
			tt.pieceExpiration,
			tt.orderExpiration,
			tt.limit,
		)

		orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
		require.NoError(t, err)

		uploader, err := client.Upload(ctx, orderLimit)
		require.NoError(t, err)

		var writeErr error
		buffer := make([]byte, memory.KiB)
		for i := 0; i < 10; i++ {
			testrand.Read(buffer)
			_, writeErr = uploader.Write(buffer)
			if writeErr != nil {
				break
			}
		}
		_, commitErr := uploader.Commit(ctx)
		err = errs.Combine(writeErr, commitErr)
		testIndex := fmt.Sprintf("#%d", i)
		if tt.err != "" {
			require.Error(t, err, testIndex)
			require.Contains(t, err.Error(), tt.err, testIndex)
		} else {
			require.NoError(t, err, testIndex)
		}
	}
}

func TestOrderLimitGetValidation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	defaultPieceSize := 10 * memory.KiB

	for _, storageNode := range planet.StorageNodes {
		err = storageNode.DB.Bandwidth().Add(ctx, planet.Satellites[0].ID(), pb.PieceAction_GET, memory.TB.Int64()-(15*memory.KiB.Int64()), time.Now())
		require.NoError(t, err)
	}

	{ // upload test piece
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		satellite := planet.Satellites[0].Identity

		orderLimit := GenerateOrderLimit(
			t,
			satellite.ID,
			planet.Uplinks[0].ID(),
			planet.StorageNodes[0].ID(),
			storj.PieceID{1},
			pb.PieceAction_PUT,
			storj.SerialNumber{0},
			oneWeek,
			oneWeek,
			defaultPieceSize.Int64(),
		)

		orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
		require.NoError(t, err)

		uploader, err := client.Upload(ctx, orderLimit)
		require.NoError(t, err)

		data := testrand.Bytes(defaultPieceSize)

		_, err = uploader.Write(data)
		require.NoError(t, err)
		_, err = uploader.Commit(ctx)
		require.NoError(t, err)
	}

	for _, tt := range []struct {
		satellite       *identity.FullIdentity
		pieceID         storj.PieceID
		action          pb.PieceAction
		serialNumber    storj.SerialNumber
		pieceExpiration time.Duration
		orderExpiration time.Duration
		limit           int64
		err             string
	}{
		{ // allocated bandwidth limit
			pieceID:         storj.PieceID{1},
			action:          pb.PieceAction_GET,
			serialNumber:    storj.SerialNumber{1},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           10 * memory.KiB.Int64(),
			err:             "out of bandwidth",
		},
	} {
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)
		defer ctx.Check(client.Close)

		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		satellite := planet.Satellites[0].Identity
		if tt.satellite != nil {
			signer = signing.SignerFromFullIdentity(tt.satellite)
			satellite = tt.satellite
		}

		orderLimit := GenerateOrderLimit(
			t,
			satellite.ID,
			planet.Uplinks[0].ID(),
			planet.StorageNodes[0].ID(),
			tt.pieceID,
			tt.action,
			tt.serialNumber,
			tt.pieceExpiration,
			tt.orderExpiration,
			tt.limit,
		)

		orderLimit, err = signing.SignOrderLimit(ctx, signer, orderLimit)
		require.NoError(t, err)

		downloader, err := client.Download(ctx, orderLimit, 0, tt.limit)
		require.NoError(t, err)

		var readErr error
		buffer := make([]byte, memory.KiB)
		for i := 0; i < 10; i++ {
			_, readErr = downloader.Read(buffer)
			if readErr != nil {
				break
			}
		}
		closeErr := downloader.Close()
		err = errs.Combine(readErr, closeErr)
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)
		}
	}
}

func setBandwidth(ctx context.Context, t *testing.T, planet *testplanet.Planet, bandwidth int64) {
	if bandwidth == 0 {
		return
	}
	for _, storageNode := range planet.StorageNodes {
		availableBandwidth, err := storageNode.Storage2.Monitor.AvailableBandwidth(ctx)
		require.NoError(t, err)
		diff := (bandwidth - availableBandwidth) * -1
		err = storageNode.DB.Bandwidth().Add(ctx, planet.Satellites[0].ID(), pb.PieceAction_GET, diff, time.Now())
		require.NoError(t, err)
	}
}

func setSpace(ctx context.Context, t *testing.T, planet *testplanet.Planet, space int64) {
	if space == 0 {
		return
	}
	for _, storageNode := range planet.StorageNodes {
		availableSpace, err := storageNode.Storage2.Monitor.AvailableSpace(ctx)
		require.NoError(t, err)
		diff := (space - availableSpace) * -1
		now := time.Now()
		err = storageNode.DB.PieceInfo().Add(ctx, &pieces.Info{
			SatelliteID:     planet.Satellites[0].ID(),
			PieceID:         storj.PieceID{99},
			PieceSize:       diff,
			PieceCreation:   now,
			PieceExpiration: time.Time{},
			Uplink:          planet.Uplinks[0].Identity.PeerIdentity(),
			UplinkPieceHash: &pb.PieceHash{},
		})
		require.NoError(t, err)
	}
}
