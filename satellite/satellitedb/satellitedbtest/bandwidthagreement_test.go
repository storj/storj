// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
)

func TestBandwidthAgreement(t *testing.T) {
	Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		require.NoError(t, testCreateAgreement(ctx, t, db.BandwidthAgreement(), pb.BandwidthAction_PUT, "1"))
		require.Error(t, testCreateAgreement(ctx, t, db.BandwidthAgreement(), pb.BandwidthAction_GET, "1"))
		require.NoError(t, testCreateAgreement(ctx, t, db.BandwidthAgreement(), pb.BandwidthAction_GET, "2"))
		testGetTotals(ctx, t, db.BandwidthAgreement())
		testGetUplinkStats(ctx, t, db.BandwidthAgreement())
	})
}

func testCreateAgreement(ctx context.Context, t *testing.T, b bwagreement.DB, action pb.BandwidthAction, serialNum string) error {
	rba := &pb.RenterBandwidthAllocation{
		PayerAllocation: pb.PayerBandwidthAllocation{Action: action, SerialNumber: serialNum},
		Total:           1000,
	}
	return b.CreateAgreement(ctx, rba)
}

func testGetUplinkStats(ctx context.Context, t *testing.T, b bwagreement.DB) {
	stats, err := b.GetUplinkStats(ctx, time.Time{}, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, storj.NodeID{}, stats[0].NodeID)
	require.Equal(t, int64(2000), stats[0].TotalBytes)
	require.Equal(t, 1, stats[0].GetActionCount)
	require.Equal(t, 1, stats[0].PutActionCount)
	require.Equal(t, 2, stats[0].TotalTransactions)
}

func testGetTotals(ctx context.Context, t *testing.T, b bwagreement.DB) {
	totals, err := b.GetTotals(ctx, time.Time{}, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, totals, 1)
	require.Len(t, totals[storj.NodeID{}], 5)
	expected := []int64{1000, 1000, 0, 0, 0}
	for i, e := range expected {
		require.Equal(t, e, totals[storj.NodeID{}][i])
	}
}
