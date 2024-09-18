// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/location"
)

// TestCheckIn ensures that redundant node check-ins aren't sent to the database.
// This is verified by comparing the last contact time from the database with
// the time of the unnecessary check-in.
func TestCheckIn(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.GeoIP.MockCountries = []string{"US"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		wait := sat.Config.Overlay.NodeCheckInWaitPeriod

		nodeID := testrand.NodeID()
		oldInfo, err := sat.Overlay.Service.Get(ctx, nodeID)
		require.Error(t, overlay.ErrNodeNotFound.New("%v", nodeID), err)
		require.Nil(t, oldInfo)

		nodeInfo := overlay.NodeCheckInInfo{
			NodeID:     nodeID,
			Address:    &pb.NodeAddress{Address: "127.0.1.0"},
			LastNet:    "127.0.1",
			LastIPPort: "127.0.1.0:8080",
			IsUp:       true,
			Operator: &pb.NodeOperator{
				Wallet:         "0x" + strings.Repeat("00", 20),
				Email:          "abc123@mail.test",
				WalletFeatures: []string{},
			},
			Capacity: &pb.NodeCapacity{},
			Version:  &pb.NodeVersion{Version: "v1.0.0"},
		}

		now := time.Now()
		lastFail := time.Time{}

		// infoCheck sends a node check-in and gets the node's info.
		// The last contact timestamp is compared to the expected timestamp.
		infoCheck := func(testName string, checkTime time.Time, expectedLastSuccess time.Time, expectedLastFailure time.Time) {
			require.NoErrorf(t, sat.Overlay.Service.UpdateCheckIn(ctx, nodeInfo, checkTime), testName)

			oldInfo, err := sat.Overlay.Service.Get(ctx, nodeID)
			require.NoErrorf(t, err, testName)

			require.Equal(t, expectedLastSuccess.Truncate(time.Second).UTC(),
				oldInfo.Reputation.LastContactSuccess.Truncate(time.Second).UTC(), testName)

			require.Equal(t, expectedLastFailure.Truncate(time.Second).UTC(),
				oldInfo.Reputation.LastContactFailure.Truncate(time.Second).UTC(), testName)

			require.Equal(t, location.UnitedStates, oldInfo.CountryCode)
		}

		infoCheck("First check-in", now, now, lastFail)

		infoCheck("Within wait period - no information changed", now.Add(wait-time.Minute), now, lastFail)

		now = now.Add(wait + time.Minute)
		infoCheck("After wait period - no information changed", now, now, lastFail)

		now = now.Add(time.Second)
		lastFail = now
		nodeInfo.IsUp = false
		infoCheck("Within wait period - node taken offline", now, now.Add(-time.Second), lastFail)

		now = now.Add(time.Second)
		nodeInfo.IsUp = true
		infoCheck("Within wait period - node back online", now, now, lastFail)

		now = now.Add(time.Second)
		nodeInfo.Address.Address = "127.0.2.0"
		infoCheck("Within wait period - changed: Address", now, now, lastFail)

		now = now.Add(time.Second)
		nodeInfo.Operator.Wallet = "0x" + strings.Repeat("11", 20)
		infoCheck("Within wait period - changed: Wallet", now, now, lastFail)

		now = now.Add(time.Second)
		nodeInfo.LastNet = "127.0.2"
		infoCheck("Within wait period - changed: LastNet", now, now, lastFail)

		now = now.Add(time.Second)
		nodeInfo.LastIPPort = "127.0.2.0:8080"
		infoCheck("Within wait period - changed: LastIPPort", now, now, lastFail)

		now = now.Add(time.Second)
		nodeInfo.Version.Version = "v2.0.0"
		infoCheck("Within wait period - changed: Version", now, now, lastFail)

		now = now.Add(time.Second)
		nodeInfo.Capacity.FreeDisk = 1
		infoCheck("Within wait period - changed: FreeDisk", now, now, lastFail)
	})
}
