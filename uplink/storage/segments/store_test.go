// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/storage/segments"
)

func TestCalcNeededNodes(t *testing.T) {
	for i, tt := range []struct {
		k, m, o, n int16
		needed     int32
	}{
		{k: 0, m: 0, o: 0, n: 0, needed: 0},
		{k: 1, m: 1, o: 1, n: 1, needed: 1},
		{k: 1, m: 1, o: 2, n: 2, needed: 2},
		{k: 1, m: 2, o: 2, n: 2, needed: 2},
		{k: 2, m: 3, o: 4, n: 4, needed: 3},
		{k: 2, m: 4, o: 6, n: 8, needed: 3},
		{k: 20, m: 30, o: 40, n: 50, needed: 25},
		{k: 29, m: 35, o: 80, n: 95, needed: 34},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		rs := storj.RedundancyScheme{
			RequiredShares: tt.k,
			RepairShares:   tt.m,
			OptimalShares:  tt.o,
			TotalShares:    tt.n,
		}

		assert.Equal(t, tt.needed, segments.CalcNeededNodes(rs), tag)
	}
}

// func runTest(t *testing.T, test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, streamStore streams.Store)) {
// 	testplanet.Run(t, testplanet.Config{
// 		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
// 	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
// 		// TODO move apikey creation to testplanet
// 		project, err := planet.Satellites[0].DB.Console().Projects().Insert(context.Background(), &console.Project{
// 			Name: "testProject",
// 		})
// 		require.NoError(t, err)

// 		apiKey, err := macaroon.NewAPIKey([]byte("testSecret"))
// 		require.NoError(t, err)

// 		apiKeyInfo := console.APIKeyInfo{
// 			ProjectID: project.ID,
// 			Name:      "testKey",
// 			Secret:    []byte("testSecret"),
// 		}

// 		// add api key to db
// 		_, err = planet.Satellites[0].DB.Console().APIKeys().Create(context.Background(), apiKey.Head(), apiKeyInfo)
// 		require.NoError(t, err)

// 		TestAPIKey := apiKey.Serialize()

// 		metainfo, err := planet.Uplinks[0].DialMetainfo(context.Background(), planet.Satellites[0], TestAPIKey)
// 		require.NoError(t, err)
// 		defer ctx.Check(metainfo.Close)

// 		ec := ecclient.NewClient(planet.Uplinks[0].Log.Named("ecclient"), planet.Uplinks[0].Transport, 0)
// 		fc, err := infectious.NewFEC(2, 4)
// 		require.NoError(t, err)

// 		rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, 1*memory.KiB.Int()), 0, 0)
// 		require.NoError(t, err)

// 		segmentStore := segments.NewSegmentStore(metainfo, ec, rs, 4*memory.KiB.Int(), 8*memory.MiB.Int64())
// 		assert.NotNil(t, segmentStore)

// 		streamStore, err := streams.NewStreamStore(metainfo, segmentStore, cfg.Volatile.SegmentsSize.Int64(), access.store, int(encryptionParameters.BlockSize), encryptionParameters.CipherSuite, p.maxInlineSize.Int())
// 		if err != nil {
// 			return nil, err
// 		}

// 		test(t, ctx, planet, streamStore)
// 	})
// }
