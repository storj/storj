// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package usedserials_test

import (
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/piecestore/usedserials"
)

type Serial struct {
	SatelliteID         storj.NodeID
	SerialNumber        storj.SerialNumber
	PartialSerialNumber usedserials.Partial
	Expiration          time.Time
}

func TestUsedSerials(t *testing.T) {
	ctx := testcontext.New(t)

	usedSerials := usedserials.NewTable(memory.MiB)

	node0 := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())
	node1 := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())

	serial1 := testrand.SerialNumber()
	serial2 := testrand.SerialNumber()
	serial3 := testrand.SerialNumber()
	serial4 := testrand.SerialNumber()
	serial5 := testrand.SerialNumber()

	var partialSerial1, partialSerial2, partialSerial3, partialSerial4, partialSerial5 usedserials.Partial
	copy(partialSerial1[:], serial1[8:])
	copy(partialSerial2[:], serial2[8:])
	copy(partialSerial3[:], serial3[8:])
	copy(partialSerial4[:], serial4[8:])
	copy(partialSerial5[:], serial5[8:])

	now := time.Now()

	// queries on empty table
	usedSerials.DeleteExpired(ctx, now.Add(6*time.Minute))
	require.Zero(t, usedSerials.Count())

	// let's start adding data
	// use different timezones
	location := time.FixedZone("XYZ", int((8 * time.Hour).Seconds()))

	// the serials with expiration times embedded are based on serial4 and serial5
	serialWithExp1 := createExpirationSerial(serial4, now.Add(8*time.Hour))
	serialWithExp2 := createExpirationSerial(serial5, now.Add(time.Hour))

	serialNumbers := []Serial{
		{node0.ID, serial1, partialSerial1, now.Add(time.Hour)},
		{node0.ID, serial2, partialSerial2, now.Add(4 * time.Hour)},
		{node0.ID, serial3, partialSerial3, now.In(location).Add(8 * time.Hour)},
		{node1.ID, serial1, partialSerial1, now.In(location).Add(time.Hour)},
		{node1.ID, serial2, partialSerial2, now.Add(4 * time.Hour)},
		{node1.ID, serial3, partialSerial3, now.Add(8 * time.Hour)},

		{node0.ID, serialWithExp1, partialSerial4, now.Add(8 * time.Hour)},
		{node0.ID, serialWithExp2, partialSerial5, now.Add(time.Hour)},
		{node1.ID, serialWithExp1, partialSerial4, now.Add(8 * time.Hour)},
		{node1.ID, serialWithExp2, partialSerial5, now.Add(time.Hour)},
	}

	// basic adding
	for _, serial := range serialNumbers {
		err := usedSerials.Add(ctx, serial.SatelliteID, serial.SerialNumber, serial.Expiration)
		require.NoError(t, err)
	}

	// duplicate adds should fail
	for _, serial := range serialNumbers {
		err := usedSerials.Add(ctx, serial.SatelliteID, serial.SerialNumber, serial.Expiration)
		require.Error(t, err)
		require.True(t, usedserials.ErrSerialAlreadyExists.Has(err))
	}

	// ensure all the serials exist
	require.Equal(t, len(serialNumbers), usedSerials.Count())
	for _, serial := range serialNumbers {
		require.True(t, usedSerials.Exists(serial.SatelliteID, serial.SerialNumber, serial.Expiration))
	}

	// ensure we can delete expired
	usedSerials.DeleteExpired(ctx, now.Add(6*time.Hour))

	// check that we have actually deleted things
	expectedAfterDelete := []Serial{
		{node0.ID, serial3, partialSerial3, now.Add(8 * time.Hour)},
		{node1.ID, serial3, partialSerial3, now.Add(8 * time.Hour)},
		{node0.ID, serialWithExp1, partialSerial4, now.Add(8 * time.Hour)},
		{node1.ID, serialWithExp1, partialSerial4, now.Add(8 * time.Hour)},
	}

	require.Equal(t, len(expectedAfterDelete), usedSerials.Count())
	for _, serial := range expectedAfterDelete {
		require.True(t, usedSerials.Exists(serial.SatelliteID, serial.SerialNumber, serial.Expiration))
	}
}

// TestUsedSerialsMemory ensures that random serials are deleted if the allocated memory size is exceeded.
func TestUsedSerialsMemory(t *testing.T) {
	ctx := testcontext.New(t)

	// first, test with partial serial numbers
	entrySize := usedserials.PartialSize

	// allow for up to three items
	// add one byte so that we don't remove items at exactly the threshold when adding a duplicate.
	usedSerials := usedserials.NewTable(3 * entrySize)
	require.Zero(t, usedSerials.Count())

	for i := 0; i < 10; i++ {
		newNodeID := testrand.NodeID()
		expiration := time.Now().Add(time.Hour)
		newSerial := createExpirationSerial(testrand.SerialNumber(), expiration)

		err := usedSerials.Add(ctx, newNodeID, newSerial, expiration)
		require.NoError(t, err)

		expectedCount := 3
		if i < 2 {
			expectedCount = i + 1
		}

		// expect count to be correct
		require.EqualValues(t, expectedCount, usedSerials.Count())
	}

	// now, test with full serial numbers
	entrySize = usedserials.FullSize

	// allow for up to three items
	usedSerials = usedserials.NewTable(3 * entrySize)
	require.Zero(t, usedSerials.Count())

	for i := 0; i < 10; i++ {
		newNodeID := testrand.NodeID()
		expiration := time.Now().Add(time.Hour)
		newSerial := testrand.SerialNumber()

		err := usedSerials.Add(ctx, newNodeID, newSerial, expiration)
		require.NoError(t, err)

		expectedCount := 3
		if i < 2 {
			expectedCount = i + 1
		}

		// expect count to be correct
		require.EqualValues(t, expectedCount, usedSerials.Count())
	}
}

func createExpirationSerial(originalSerial storj.SerialNumber, expiration time.Time) storj.SerialNumber {
	serialWithExp := storj.SerialNumber{}
	copy(serialWithExp[:], originalSerial[:])
	// make first 8 bytes of serial expiration so that it is stored as a partial serial
	binary.BigEndian.PutUint64(serialWithExp[0:8], uint64(expiration.Unix()))

	return serialWithExp
}

func Benchmark(b *testing.B) {
	ctx := testcontext.New(b)
	size := memory.MiB
	used := usedserials.NewTable(size)
	nodeID := testrand.NodeID()

	r := rand.NewSource(0)
	now := time.Now().Add(10 * 24 * time.Hour)
	insertRandom := func() {
		var serial storj.SerialNumber
		v := r.Int63()
		binary.LittleEndian.PutUint64(serial[:8], uint64(v))
		binary.LittleEndian.PutUint64(serial[8:], uint64(v))
		_ = used.Add(ctx, nodeID, serial, now)
	}

	// fill the used table
	for i := 0; i < int(size/usedserials.FullSize); i++ {
		insertRandom()
	}

	b.Run("AddDelete", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			insertRandom()
		}
	})

}
