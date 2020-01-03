// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

const oneWeek = 7 * 24 * time.Hour

func TestOrderLimitPutValidation(t *testing.T) {
	for _, tt := range []struct {
		testName            string
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
		{
			testName:            "unapproved satellite id",
			useUnknownSatellite: true,
			pieceID:             storj.PieceID{1},
			action:              pb.PieceAction_PUT,
			serialNumber:        storj.SerialNumber{1},
			pieceExpiration:     oneWeek,
			orderExpiration:     oneWeek,
			limit:               memory.KiB.Int64(),
			err:                 " is untrusted",
		},
		{
			testName:        "approved satellite id",
			pieceID:         storj.PieceID{2},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{2},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           10 * memory.KiB.Int64(),
		},
		{
			testName:        "wrong action type",
			pieceID:         storj.PieceID{3},
			action:          pb.PieceAction_GET,
			serialNumber:    storj.SerialNumber{3},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           memory.KiB.Int64(),
			err:             "expected put or put repair action got GET",
		},
		{
			testName:        "piece expired",
			pieceID:         storj.PieceID{4},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{4},
			pieceExpiration: -4 * 24 * time.Hour,
			orderExpiration: oneWeek,
			limit:           memory.KiB.Int64(),
			err:             "piece expired:",
		},
		{
			testName:        "limit is negative",
			pieceID:         storj.PieceID{5},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{5},
			pieceExpiration: oneWeek,
			orderExpiration: oneWeek,
			limit:           -1,
			err:             "order limit is negative",
		},
		{
			testName:        "order limit expired",
			pieceID:         storj.PieceID{6},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{6},
			pieceExpiration: oneWeek,
			orderExpiration: -4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
			err:             "order expired:",
		},
		{
			testName:           "allocated bandwidth limit",
			pieceID:            storj.PieceID{7},
			action:             pb.PieceAction_PUT,
			serialNumber:       storj.SerialNumber{7},
			pieceExpiration:    oneWeek,
			orderExpiration:    oneWeek,
			limit:              10 * memory.KiB.Int64(),
			availableBandwidth: 5 * memory.KiB.Int64(),
			err:                "out of bandwidth",
		},
		{
			testName:        "allocated space limit",
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
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
			}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

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

				orderLimit, piecePrivateKey := GenerateOrderLimit(
					t,
					satellite.ID,
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

				uploader, err := client.Upload(ctx, orderLimit, piecePrivateKey)
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
				if tt.err != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.err)
				} else {
					require.NoError(t, err)
				}
			})
		})
	}
}

func TestOrderLimitGetValidation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		defaultPieceSize := 10 * memory.KiB

		for _, storageNode := range planet.StorageNodes {
			err := storageNode.DB.Bandwidth().Add(ctx, planet.Satellites[0].ID(), pb.PieceAction_GET, memory.TB.Int64()-(15*memory.KiB.Int64()), time.Now())
			require.NoError(t, err)
		}

		{ // upload test piece
			client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
			require.NoError(t, err)
			defer ctx.Check(client.Close)

			signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
			satellite := planet.Satellites[0].Identity

			orderLimit, piecePrivateKey := GenerateOrderLimit(
				t,
				satellite.ID,
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

			uploader, err := client.Upload(ctx, orderLimit, piecePrivateKey)
			require.NoError(t, err)

			data := testrand.Bytes(defaultPieceSize)

			_, err = uploader.Write(data)
			require.NoError(t, err)
			_, err = uploader.Commit(ctx)
			require.NoError(t, err)
		}

		// wait for all requests to finish to ensure that the upload usage has been
		// accounted for.
		waitForEndpointRequestsToDrain(t, planet)

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

			orderLimit, piecePrivateKey := GenerateOrderLimit(
				t,
				satellite.ID,
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

			downloader, err := client.Download(ctx, orderLimit, piecePrivateKey, 0, tt.limit)
			require.NoError(t, err)

			buffer, readErr := ioutil.ReadAll(downloader)
			closeErr := downloader.Close()
			err = errs.Combine(readErr, closeErr)
			if tt.err != "" {
				assert.Equal(t, 0, len(buffer))
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)
			}
		}
	})
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
		// add these bytes to the space used cache so that we can test what happens
		// when we exceeded available space on the storagenode
		err = storageNode.DB.PieceSpaceUsedDB().UpdatePieceTotal(ctx, availableSpace-space)
		require.NoError(t, err)
		err = storageNode.Storage2.CacheService.Init(ctx)
		require.NoError(t, err)
	}
}

func waitForEndpointRequestsToDrain(t *testing.T, planet *testplanet.Planet) {
	timeout := time.NewTimer(time.Minute)
	defer timeout.Stop()
	for {
		if endpointRequestCount(planet) == 0 {
			return
		}
		select {
		case <-time.After(50 * time.Millisecond):
		case <-timeout.C:
			require.FailNow(t, "timed out waiting for endpoint requests to drain")
		}
	}
}

func endpointRequestCount(planet *testplanet.Planet) int {
	total := 0
	for _, storageNode := range planet.StorageNodes {
		total += int(storageNode.Storage2.Endpoint.TestLiveRequestCount())
	}
	return total
}
