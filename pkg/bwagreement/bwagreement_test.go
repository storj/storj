// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBandwidthDBAgreement(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		upID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		snID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)

		require.NoError(t, testCreateAgreement(ctx, t, db.BandwidthAgreement(), pb.BandwidthAction_PUT, "1", upID, snID))
		require.Error(t, testCreateAgreement(ctx, t, db.BandwidthAgreement(), pb.BandwidthAction_GET, "1", upID, snID))
		require.NoError(t, testCreateAgreement(ctx, t, db.BandwidthAgreement(), pb.BandwidthAction_GET, "2", upID, snID))
		testGetTotals(ctx, t, db.BandwidthAgreement(), snID)
		testGetUplinkStats(ctx, t, db.BandwidthAgreement(), upID)
	})
}

func testCreateAgreement(ctx context.Context, t *testing.T, b bwagreement.DB, action pb.BandwidthAction,
	serialNum string, upID, snID *identity.FullIdentity) error {
	rba := &pb.RenterBandwidthAllocation{
		PayerAllocation: pb.PayerBandwidthAllocation{
			Action:       action,
			SerialNumber: serialNum,
			UplinkId:     upID.ID,
		},
		Total:         1000,
		StorageNodeId: snID.ID,
	}
	return b.CreateAgreement(ctx, rba)
}

func testGetUplinkStats(ctx context.Context, t *testing.T, b bwagreement.DB, upID *identity.FullIdentity) {
	stats, err := b.GetUplinkStats(ctx, time.Time{}, time.Now().UTC())
	require.NoError(t, err)
	var found int
	for _, s := range stats {
		if upID.ID == s.NodeID {
			found++
			require.Equal(t, int64(2000), s.TotalBytes)
			require.Equal(t, 1, s.GetActionCount)
			require.Equal(t, 1, s.PutActionCount)
			require.Equal(t, 2, s.TotalTransactions)
		}
	}
	require.Equal(t, 1, found)
}

func testGetTotals(ctx context.Context, t *testing.T, b bwagreement.DB, snID *identity.FullIdentity) {
	totals, err := b.GetTotals(ctx, time.Time{}, time.Now().UTC())
	require.NoError(t, err)
	total := totals[snID.ID]
	require.Len(t, total, 5)
	require.Equal(t, int64(1000), total[pb.BandwidthAction_PUT])
	require.Equal(t, int64(1000), total[pb.BandwidthAction_GET])
	require.Equal(t, int64(0), total[pb.BandwidthAction_GET_AUDIT])
	require.Equal(t, int64(0), total[pb.BandwidthAction_GET_REPAIR])
	require.Equal(t, int64(0), total[pb.BandwidthAction_PUT_REPAIR])
}
