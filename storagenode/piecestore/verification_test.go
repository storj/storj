// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestOrderLimitPutValidation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	unapprovedSatellite, err := planet.NewIdentity()
	require.NoError(t, err)

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
		{ // unapproved satellite id
			satellite:       unapprovedSatellite,
			pieceID:         storj.PieceID{1},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{1},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
			err:             " is untrusted",
		},
		{ // approved satellite id
			pieceID:         storj.PieceID{2},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{2},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
		},
		{ // wrong action type
			pieceID:         storj.PieceID{2},
			action:          pb.PieceAction_GET,
			serialNumber:    storj.SerialNumber{3},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
			err:             "expected put or put repair action got GET",
		},
		{ // piece expired
			pieceID:         storj.PieceID{4},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{4},
			pieceExpiration: -4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
			err:             "piece expired:",
		},
		{ // limit is negative
			pieceID:         storj.PieceID{5},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{5},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           -1,
			err:             "order limit is negative",
		},
		{ // order limit expired
			pieceID:         storj.PieceID{6},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{5},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: -4 * 24 * time.Hour,
			limit:           memory.KiB.Int64(),
			err:             "order expired:",
		},
		{ // allocated space limit
			pieceID:         storj.PieceID{7},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{7},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           2 * memory.TB.Int64(),
			err:             "out of space",
		},
		{ // allocated bandwidth limit
			pieceID:         storj.PieceID{8},
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber{8},
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           1200 * memory.GB.Int64(),
			err:             "out of bandwidth",
		},
	} {
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)

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

		orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
		require.NoError(t, err)

		uploader, err := client.Upload(ctx, orderLimit)
		require.NoError(t, err)

		data := make([]byte, 1*memory.KiB)
		_, _ = rand.Read(data)

		_, writeErr := uploader.Write(data)
		_, commitErr := uploader.Commit()
		err = errs.Combine(writeErr, commitErr)
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)
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
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           1200 * memory.GB.Int64(),
			err:             "out of bandwidth",
		},
	} {
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)

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

		orderLimit, err = signing.SignOrderLimit(signer, orderLimit)
		require.NoError(t, err)

		downloader, err := client.Download(ctx, orderLimit, 0, tt.limit)
		require.NoError(t, err)

		_, writeErr := downloader.Read([]byte{})
		closeErr := downloader.Close()
		err = errs.Combine(writeErr, closeErr)
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)
		}
	}
}
