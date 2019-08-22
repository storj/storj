// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestDeleteExpiredSerials(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0].ID()
		satellite.DBCleanup.Service.Serials.Pause()

		var expiredSerials []storj.SerialNumber
		for i := 0; i < 5; i++ {
			expiredSerials = append(expiredSerials, storj.SerialNumber{byte(i)})
		}

		var freshSerials []storj.SerialNumber
		for i := 5; i < 10; i++ {
			freshSerials = append(freshSerials, storj.SerialNumber{byte(i)})
		}

		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		for i := 0; i < 5; i++ {
			err := satellite.DB.Orders().CreateSerialInfo(ctx, expiredSerials[i], []byte("bucket"), yesterday)
			require.NoError(t, err)

			_, err = satellite.DB.Orders().UseSerialNumber(ctx, expiredSerials[i], node)
			require.NoError(t, err)
		}

		tomorrow := yesterday.Add(48 * time.Hour)
		for i := 0; i < 5; i++ {
			err := satellite.DB.Orders().CreateSerialInfo(ctx, freshSerials[i], []byte("bucket"), tomorrow)
			require.NoError(t, err)

			_, err = satellite.DB.Orders().UseSerialNumber(ctx, freshSerials[i], node)
			require.NoError(t, err)
		}

		// trigger expired serial number deletion
		satellite.DBCleanup.Service.Serials.TriggerWait()

		// check expired serial numbers have been deleted from serial_numbers and used_serials
		for i := 0; i < 5; i++ {
			_, err := satellite.DB.Orders().UseSerialNumber(ctx, expiredSerials[i], node)
			require.EqualError(t, err, "serial number: serial number not found")
		}

		// check fresh serial numbers have not been deleted
		for i := 0; i < 5; i++ {
			_, err := satellite.DB.Orders().UseSerialNumber(ctx, freshSerials[i], node)
			require.EqualError(t, err, "serial number: serial number already used")
		}
	})
}
