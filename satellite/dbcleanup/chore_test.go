// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestDeleteExpiredSerials(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0].ID()
		satellite.DBCleanup.Chore.Serials.Pause()

		var expiredSerials []storj.SerialNumber
		for i := 0; i < 5; i++ {
			expiredSerials = append(expiredSerials, storj.SerialNumber{byte(i)})
		}

		var freshSerials []storj.SerialNumber
		for i := 5; i < 10; i++ {
			freshSerials = append(freshSerials, storj.SerialNumber{byte(i)})
		}

		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		for _, serial := range expiredSerials {
			err := satellite.DB.Orders().CreateSerialInfo(ctx, serial, []byte("bucket"), yesterday)
			require.NoError(t, err)

			_, err = satellite.DB.Orders().UseSerialNumber(ctx, serial, node)
			require.NoError(t, err)
		}

		tomorrow := yesterday.Add(48 * time.Hour)
		for _, serial := range freshSerials {
			err := satellite.DB.Orders().CreateSerialInfo(ctx, serial, []byte("bucket"), tomorrow)
			require.NoError(t, err)

			_, err = satellite.DB.Orders().UseSerialNumber(ctx, serial, node)
			require.NoError(t, err)
		}

		// trigger expired serial number deletion
		satellite.DBCleanup.Chore.Serials.TriggerWait()

		// check expired serial numbers have been deleted from serial_numbers and used_serials
		for _, serial := range expiredSerials {
			_, err := satellite.DB.Orders().UseSerialNumber(ctx, serial, node)
			require.EqualError(t, err, "serial number: serial number not found")
		}

		// check fresh serial numbers have not been deleted
		for _, serial := range freshSerials {
			_, err := satellite.DB.Orders().UseSerialNumber(ctx, serial, node)
			require.EqualError(t, err, "serial number: serial number already used")
		}
	})
}
