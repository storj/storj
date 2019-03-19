// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestOrderLimitValidation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	for _, tt := range []struct {
		satelliteID     *identity.FullIdentity
		uplinkID        storj.NodeID
		storageNodeID   storj.NodeID
		pieceID         storj.PieceID
		action          pb.PieceAction
		serialNumber    storj.SerialNumber
		pieceExpiration time.Duration
		orderExpiration time.Duration
		limit           int64
		err             string
	}{
		{ // unapproved satellite id
			satelliteID:     testplanet.MustPregeneratedIdentity(0),
			pieceID:         teststorj.PieceIDFromString("piece-id-1"),
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber([16]byte{1}),
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			err:             " is untrusted",
		},
		{ // approved satellite id
			pieceID:         teststorj.PieceIDFromString("piece-id-2"),
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber([16]byte{2}),
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
		},
		{ // wrong action type
			pieceID:         teststorj.PieceIDFromString("piece-id-3"),
			action:          pb.PieceAction_GET,
			serialNumber:    storj.SerialNumber([16]byte{3}),
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			err:             "expected put or put repair action got GET",
		},
		{ // piece expired
			pieceID:         teststorj.PieceIDFromString("piece-id-4"),
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber([16]byte{4}),
			pieceExpiration: -4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			err:             "piece expired:",
		},
		{ // limit is negative
			pieceID:         teststorj.PieceIDFromString("piece-id-5"),
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber([16]byte{5}),
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: 4 * 24 * time.Hour,
			limit:           -1,
			err:             "order limit is negative",
		},
		{ // order limit expired
			pieceID:         teststorj.PieceIDFromString("piece-id-6"),
			action:          pb.PieceAction_PUT,
			serialNumber:    storj.SerialNumber([16]byte{5}),
			pieceExpiration: 4 * 24 * time.Hour,
			orderExpiration: -4 * 24 * time.Hour,
			err:             "order expired:",
		},
	} {
		client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
		require.NoError(t, err)

		signer := signing.SignerFromFullIdentity(planet.Satellites[0].Identity)
		satellite := planet.Satellites[0].Identity
		if tt.satelliteID != nil {
			signer = signing.SignerFromFullIdentity(tt.satelliteID)
			satellite = tt.satelliteID
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

		_, err = uploader.Commit()
		if tt.err != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.err)
		} else {
			require.NoError(t, err)
		}
	}
}
