// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestUsedSerials(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		usedSerials := db.UsedSerials()

		node0 := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())
		node1 := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())

		serial1 := testrand.SerialNumber()
		serial2 := testrand.SerialNumber()
		serial3 := testrand.SerialNumber()

		now := time.Now()

		// queries on empty table
		err := usedSerials.DeleteExpired(ctx, now.Add(6*time.Minute))
		assert.NoError(t, err)

		err = usedSerials.IterateAll(ctx, func(satellite storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) {})
		assert.NoError(t, err)

		// let's start adding data
		type Serial struct {
			SatelliteID  storj.NodeID
			SerialNumber storj.SerialNumber
			Expiration   time.Time
		}

		// use different timezones
		location := time.FixedZone("XYZ", int((8 * time.Hour).Seconds()))

		serialNumbers := []Serial{
			{node0.ID, serial1, now.Add(time.Minute)},
			{node0.ID, serial2, now.Add(4 * time.Minute)},
			{node0.ID, serial3, now.In(location).Add(8 * time.Minute)},
			{node1.ID, serial1, now.In(location).Add(time.Minute)},
			{node1.ID, serial2, now.Add(4 * time.Minute)},
			{node1.ID, serial3, now.Add(8 * time.Minute)},
		}

		// basic adding
		for _, serial := range serialNumbers {
			err = usedSerials.Add(ctx, serial.SatelliteID, serial.SerialNumber, serial.Expiration)
			assert.NoError(t, err)
		}

		// duplicate adds should fail
		for _, serial := range serialNumbers {
			expirationDelta := time.Duration(testrand.Intn(10)-5) * time.Hour
			err = usedSerials.Add(ctx, serial.SatelliteID, serial.SerialNumber, serial.Expiration.Add(expirationDelta))
			assert.Error(t, err)
		}

		// ensure we can list all of them
		listedNumbers := []Serial{}
		err = usedSerials.IterateAll(ctx, func(satellite storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) {
			listedNumbers = append(listedNumbers, Serial{satellite, serialNumber, expiration})
		})

		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(serialNumbers, listedNumbers))

		// ensure we can delete expired
		err = usedSerials.DeleteExpired(ctx, now.Add(6*time.Minute))
		require.NoError(t, err)

		// ensure we can list after delete
		listedAfterDelete := []Serial{}
		err = usedSerials.IterateAll(ctx, func(satellite storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) {
			listedAfterDelete = append(listedAfterDelete, Serial{satellite, serialNumber, expiration})
		})

		// check that we have actually deleted things
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff([]Serial{
			{node0.ID, serial3, now.Add(8 * time.Minute)},
			{node1.ID, serial3, now.Add(8 * time.Minute)},
		}, listedAfterDelete))
	})
}

func TestUsedSerials_Trivial(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		satelliteID, serial := testrand.NodeID(), testrand.SerialNumber()

		{ // Ensure Add works at all
			err := db.UsedSerials().Add(ctx, satelliteID, serial, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure IterateAll works at all
			err := db.UsedSerials().IterateAll(ctx, func(storj.NodeID, storj.SerialNumber, time.Time) {})
			require.NoError(t, err)
		}

		{ // Ensure DeleteExpired works at all
			err := db.UsedSerials().DeleteExpired(ctx, time.Now())
			require.NoError(t, err)
		}
	})
}
