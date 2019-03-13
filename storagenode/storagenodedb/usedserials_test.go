// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/storagenodedb"
)

func TestUsedSerials(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := storagenodedb.NewInfoInMemory()
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NoError(t, db.CreateTables(log))

	usedSerials := db.UsedSerials()

	node0 := testplanet.MustPregeneratedIdentity(0)
	node1 := testplanet.MustPregeneratedIdentity(1)

	serial1 := newRandomSerial()
	serial2 := newRandomSerial()
	serial3 := newRandomSerial()

	now := time.Now()

	// queries on empty table
	err = usedSerials.DeleteExpired(ctx, now.Add(6*time.Minute))
	assert.NoError(t, err)

	err = usedSerials.IterateAll(ctx, func(satellite storj.NodeID, serialNumber []byte, expiration time.Time) {})
	assert.NoError(t, err)

	// let's start adding data
	type Serial struct {
		SatelliteID  storj.NodeID
		SerialNumber []byte
		Expiration   time.Time
	}

	serialNumbers := []Serial{
		{node0.ID, serial1, now.Add(time.Minute)},
		{node0.ID, serial2, now.Add(4 * time.Minute)},
		{node0.ID, serial3, now.Add(8 * time.Minute)},
		{node1.ID, serial1, now.Add(time.Minute)},
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
		expirationDelta := time.Duration(rand.Intn(10)-5) * time.Hour
		err = usedSerials.Add(ctx, serial.SatelliteID, serial.SerialNumber, serial.Expiration.Add(expirationDelta))
		assert.Error(t, err)
	}

	// ensure we can list all of them
	listedNumbers := []Serial{}
	err = usedSerials.IterateAll(ctx, func(satellite storj.NodeID, serialNumber []byte, expiration time.Time) {
		listedNumbers = append(listedNumbers, Serial{satellite, serialNumber, expiration})
	})

	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(serialNumbers, listedNumbers))

	// ensure we can delete expired
	err = usedSerials.DeleteExpired(ctx, now.Add(6*time.Minute))
	require.NoError(t, err)

	// ensure we can list after delete
	listedAfterDelete := []Serial{}
	err = usedSerials.IterateAll(ctx, func(satellite storj.NodeID, serialNumber []byte, expiration time.Time) {
		listedAfterDelete = append(listedAfterDelete, Serial{satellite, serialNumber, expiration})
	})

	// check that we have actually deleted things
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff([]Serial{
		{node0.ID, serial3, now.Add(8 * time.Minute)},
		{node1.ID, serial3, now.Add(8 * time.Minute)},
	}, listedAfterDelete))
}

func newRandomSerial() []byte {
	var serial [16]byte
	_, _ = rand.Read(serial[:])
	return serial[:]
}
