// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/bwagreement/testbwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
)

func runPlanet(t *testing.T, f func(context.Context, *testplanet.Planet)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)
	planet.Start(ctx)
	f(ctx, planet)
}

func TestQueryNoAgreements(t *testing.T) {
	runPlanet(t, func(ctx context.Context, planet *testplanet.Planet) {
		tally := planet.Satellites[0].Accounting.Tally
		tallyEnd, bwTotals, err := tally.QueryBW(ctx)
		require.NoError(t, err)
		err = tally.SaveBWRaw(ctx, tallyEnd, bwTotals)
		require.NoError(t, err)
	})
}

func TestQueryWithBw(t *testing.T) {
	runPlanet(t, func(ctx context.Context, planet *testplanet.Planet) {
		tally := planet.Satellites[0].Accounting.Tally
		makeBWAs(ctx, t, planet)
		//check the db
		tallyEnd, bwTotals, err := tally.QueryBW(ctx)
		require.NoError(t, err)
		require.Len(t, bwTotals, 5)
		for action, _ := range bwTotals {
			for id, total := range bwTotals[action] {
				require.Equal(t, id, planet.StorageNodes[0].Identity.ID)
				require.Equal(t, total, 1000)
			}
		}
		err = tally.SaveBWRaw(ctx, tallyEnd, bwTotals)
		require.NoError(t, err)
	})
}

func makeBWAs(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
	satID := planet.Satellites[0].Identity
	upID := planet.Uplinks[0].Identity
	snID := planet.StorageNodes[0].Identity
	sender := planet.StorageNodes[0].Agreements.Sender
	actions := []pb.BandwidthAction{pb.BandwidthAction_PUT, pb.BandwidthAction_GET,
		pb.BandwidthAction_GET_AUDIT, pb.BandwidthAction_GET_REPAIR, pb.BandwidthAction_PUT_REPAIR}
	agreements := make([]*psdb.Agreement, len(actions))
	for i, action := range actions {
		pba, err := testbwagreement.GeneratePayerBandwidthAllocation(action, satID, upID, time.Hour)
		require.NoError(t, err)
		rba, err := testbwagreement.GenerateRenterBandwidthAllocation(pba, snID.ID, upID, 1000)
		require.NoError(t, err)
		agreements[i] = &psdb.Agreement{Agreement: *rba}
	}
	sender.SendAgreementsToSatellite(ctx, snID.ID, agreements)
}
