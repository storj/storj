// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement/testbwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/satellite"
)

func TestLatestTallyForBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketTallies := generateBucketTallies(5, 10)
		before := time.Now()
		err := planet.Satellites[0].DB.Accounting().SaveBucketTallies(ctx, before, bucketTallies)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			id := strconv.Itoa(i)
			latestTally, interval, err := planet.Satellites[0].DB.Accounting().LatestTallyForBucket(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, before.UTC(), interval.UTC())
			assert.Equal(t, bucketTallies[id], latestTally)
		}

		// generate new bucket tallies with same ids but different data
		newBucketTallies := generateBucketTallies(5, 50)
		later := before.Add(time.Hour * 1)
		err = planet.Satellites[0].DB.Accounting().SaveBucketTallies(ctx, later, newBucketTallies)
		require.NoError(t, err)

		// assert LatestTallyForBucket gets latest tally data
		for i := 0; i < 5; i++ {
			id := strconv.Itoa(i)
			latestTally, interval, err := planet.Satellites[0].DB.Accounting().LatestTallyForBucket(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, later.UTC(), interval.UTC())
			assert.Equal(t, newBucketTallies[id], latestTally)
		}
	})
}

func generateBucketTallies(n int, data int64) map[string]*accounting.BucketTally {
	bucketTallies := make(map[string]*accounting.BucketTally)
	for i := 0; i < n; i++ {
		bt := &accounting.BucketTally{
			Segments:        data * 2,
			InlineSegments:  data,
			RemoteSegments:  data,
			UnknownSegments: 0,
			Files:           data * 2,
			InlineFiles:     data,
			RemoteFiles:     data,
			Bytes:           data * 2,
			InlineBytes:     data,
			RemoteBytes:     data,
			MetadataSize:    data,
		}
		bucketTallies[strconv.Itoa(i)] = bt
	}
	return bucketTallies
}
func TestQueryNoAgreements(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tally := planet.Satellites[0].Accounting.Tally
		_, bwTotals, err := tally.QueryBW(ctx)
		require.NoError(t, err)
		require.Len(t, bwTotals, 0)
	})
}

func TestQueryWithBw(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].DB
		sendGeneratedAgreements(ctx, t, db, planet)

		tally := planet.Satellites[0].Accounting.Tally
		tallyEnd, bwTotals, err := tally.QueryBW(ctx)
		require.NoError(t, err)
		require.Len(t, bwTotals, 1)

		for id, nodeTotals := range bwTotals {
			require.Len(t, nodeTotals, 5)
			for _, total := range nodeTotals {
				require.Equal(t, planet.StorageNodes[0].Identity.ID, id)
				require.Equal(t, int64(1000), total)
			}
		}
		err = tally.SaveBWRaw(ctx, tallyEnd, time.Now().UTC(), bwTotals)
		require.NoError(t, err)
	})
}

func sendGeneratedAgreements(ctx context.Context, t *testing.T, db satellite.DB, planet *testplanet.Planet) {
	satID := planet.Satellites[0].Identity
	upID := planet.Uplinks[0].Identity
	snID := planet.StorageNodes[0].Identity
	sender := planet.StorageNodes[0].Agreements.Sender
	actions := []pb.BandwidthAction{
		pb.BandwidthAction_PUT,
		pb.BandwidthAction_GET,
		pb.BandwidthAction_GET_AUDIT,
		pb.BandwidthAction_GET_REPAIR,
		pb.BandwidthAction_PUT_REPAIR,
	}

	agreements := make([]*psdb.Agreement, len(actions))
	for i, action := range actions {
		pba, err := testbwagreement.GenerateOrderLimit(action, satID, upID, time.Hour)
		require.NoError(t, err)
		err = db.CertDB().SavePublicKey(ctx, pba.UplinkId, upID.Leaf.PublicKey)
		assert.NoError(t, err)
		rba, err := testbwagreement.GenerateOrder(pba, snID.ID, upID, 1000)
		require.NoError(t, err)
		agreements[i] = &psdb.Agreement{Agreement: *rba}
	}

	sender.SettleAgreements(ctx, satID.ID, agreements)
}
