// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
)

func PieceExpirationFunctionalityTest(ctx context.Context, t *testing.T, expireDB PieceExpirationDB) {
	// test GetExpired, SetExpiration, DeleteExpiration
	satelliteID := testrand.NodeID()
	pieceID := testrand.PieceID()

	// GetExpired with no matches
	expired, err := expireDB.GetExpired(ctx, time.Now(), -1)
	require.NoError(t, err)
	require.Len(t, expired, 0)

	// DeleteExpiration with no matches
	err = expireDB.DeleteExpirations(ctx, time.Time{})
	require.NoError(t, err)

	expireAt := time.Now()

	// SetExpiration normal usage
	err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt, -1)
	require.NoError(t, err)

	// SetExpiration duplicate
	err = expireDB.SetExpiration(ctx, satelliteID, pieceID, expireAt.Add(-time.Hour), -1)
	require.NoError(t, err)

	// GetExpired normal usage
	expired, err = expireDB.GetExpired(ctx, expireAt, -1)
	require.NoError(t, err)
	require.Len(t, expired, 1)

	// DeleteExpiration normal usage
	err = expireDB.DeleteExpirations(ctx, expireAt.Add(time.Hour))
	require.NoError(t, err)

	// Should not be there anymore
	expired, err = expireDB.GetExpired(ctx, expireAt.Add(365*24*time.Hour), -1)
	require.NoError(t, err)
	require.Len(t, expired, 0)
}

func TestPieceExpirationStore(t *testing.T) {
	ctx := testcontext.New(t)

	store, err := NewPieceExpirationStore(zaptest.NewLogger(t), nil, PieceExpirationConfig{
		DataDir:               ctx.Dir("pieceexpiration"),
		ConcurrentFileHandles: 10,
	})
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	PieceExpirationFunctionalityTest(ctx, t, store)
}

func TestPieceExpirationStoreInDepth(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dataDir := ctx.Dir("pieceexpiration")
	store, err := NewPieceExpirationStore(zaptest.NewLogger(t), nil, PieceExpirationConfig{
		DataDir:               dataDir,
		ConcurrentFileHandles: 2,
	})
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	const numSatellites = 2
	const numPieces = 2

	satellites := make([]storj.NodeID, numSatellites)
	expirePieces := make([]storj.PieceID, numPieces)
	now := time.Now()

	for i := range satellites {
		satellites[i] = testrand.NodeID()
	}
	for i := range expirePieces {
		expirePieces[i] = testrand.PieceID()
	}
	sort.Slice(satellites, func(i, j int) bool { return satellites[i].Less(satellites[j]) })
	sort.Slice(expirePieces, func(i, j int) bool { return bytes.Compare(expirePieces[i][:], expirePieces[j][:]) < 0 })

	for i := range satellites {
		for p := range expirePieces {
			err = store.SetExpiration(ctx, satellites[i], expirePieces[p], now.Add(time.Duration(i)*time.Hour), int64(100*i+p+1))
			require.NoError(t, err)
		}
	}

	afterExpirations := now.Add(time.Duration(numSatellites*numPieces+1) * time.Hour)
	expirationInfos, err := store.GetExpired(ctx, afterExpirations, -1)
	require.NoError(t, err)

	require.Len(t, expirationInfos, numSatellites*numPieces)
	sort.Slice(expirationInfos, func(i, j int) bool {
		cmp := expirationInfos[i].SatelliteID.Compare(expirationInfos[j].SatelliteID)
		if cmp < 0 {
			return true
		}
		if cmp > 0 {
			return false
		}
		return bytes.Compare(expirationInfos[i].PieceID[:], expirationInfos[j].PieceID[:]) < 0
	})

	for i := range satellites {
		for p := range expirePieces {
			require.Equal(t, satellites[i], expirationInfos[i*numPieces+p].SatelliteID)
			require.Equal(t, expirePieces[p], expirationInfos[i*numPieces+p].PieceID)
			require.Equal(t, int64(100*i+p+1), expirationInfos[i*numPieces+p].PieceSize)
		}
	}

	err = store.DeleteExpirationsForSatellite(ctx, satellites[0], afterExpirations)
	require.NoError(t, err)

	expirationInfos, err = store.GetExpired(ctx, afterExpirations, -1)
	require.NoError(t, err)

	require.Len(t, expirationInfos, (numSatellites-1)*numPieces)
	for i := range satellites {
		for p := range expirePieces {
			if i == 0 {
				continue
			}
			require.Equal(t, satellites[i], expirationInfos[(i-1)*numPieces+p].SatelliteID)
			require.Equal(t, expirePieces[p], expirationInfos[(i-1)*numPieces+p].PieceID)
		}
	}

	err = store.DeleteExpirations(ctx, afterExpirations)
	require.NoError(t, err)

	expirationInfos, err = store.GetExpired(ctx, afterExpirations, -1)
	require.NoError(t, err)

	require.Len(t, expirationInfos, 0)
}

func TestPieceExpirationStoreFileTruncation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dataDir := ctx.Dir("pieceexpiration")
	store, err := NewPieceExpirationStore(zaptest.NewLogger(t), nil, PieceExpirationConfig{
		DataDir:               dataDir,
		ConcurrentFileHandles: 2,
	})
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	satelliteID := testrand.NodeID()
	pieceID1 := testrand.PieceID()
	pieceID2 := testrand.PieceID()
	now := time.Now()

	satelliteDir := PathEncoding.EncodeToString(satelliteID[:])

	dataFile := store.fileForKey(makeHourKey(satelliteID, now))
	err = os.MkdirAll(filepath.Join(dataDir, satelliteDir), 0o700)
	require.NoError(t, err)

	f, err := os.OpenFile(dataFile, os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)

	// write a full piece expiration record for pieceID1
	n, err := f.Write(pieceID1[:])
	require.NoError(t, err)
	require.Equal(t, len(storj.PieceID{}), n)
	sizeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBytes, 100)
	n, err = f.Write(sizeBytes)
	require.NoError(t, err)
	require.Equal(t, 8, n)

	// but only a part expiration record for pieceID2
	n, err = f.Write(pieceID2[:])
	require.NoError(t, err)
	require.Equal(t, len(storj.PieceID{}), n)

	require.NoError(t, f.Close())

	// check all piece expirations in store
	expirationInfos, err := store.GetExpired(ctx, now.Add(time.Hour), -1)
	require.NoError(t, err)
	require.Len(t, expirationInfos, 1)
	require.Equal(t, satelliteID, expirationInfos[0].SatelliteID)
	require.Equal(t, pieceID1, expirationInfos[0].PieceID)
	require.Equal(t, int64(100), expirationInfos[0].PieceSize)

	// append another piece expiration record for pieceID2 through the store,
	// to ensure that the result is still not corrupted
	err = store.SetExpiration(ctx, satelliteID, pieceID2, now, 200)
	require.NoError(t, err)

	expirationInfos, err = store.GetExpired(ctx, now.Add(time.Hour), -1)
	require.NoError(t, err)
	require.Len(t, expirationInfos, 2)
	require.Equal(t, satelliteID, expirationInfos[0].SatelliteID)
	require.Equal(t, pieceID1, expirationInfos[0].PieceID)
	require.Equal(t, int64(100), expirationInfos[0].PieceSize)
	require.Equal(t, satelliteID, expirationInfos[1].SatelliteID)
	require.Equal(t, pieceID2, expirationInfos[1].PieceID)
	require.Equal(t, int64(200), expirationInfos[1].PieceSize)
}

func TestPieceExpirationPeriodicFlushing(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dataDir := ctx.Dir("pieceexpiration")
	store, err := NewPieceExpirationStore(zaptest.NewLogger(t), nil, PieceExpirationConfig{
		DataDir:               dataDir,
		ConcurrentFileHandles: 2,
		MaxBufferTime:         100 * time.Millisecond,
	})
	require.NoError(t, err)
	defer ctx.Check(store.Close)

	satelliteID := testrand.NodeID()
	pieceID := testrand.PieceID()
	now := time.Now()

	err = store.SetExpiration(ctx, satelliteID, pieceID, now.Add(24*time.Hour), 123)
	require.NoError(t, err)

	dataFile := store.fileForKey(makeHourKey(satelliteID, now.Add(24*time.Hour)))
	time.Sleep(150 * time.Millisecond)
	st, err := os.Stat(dataFile)
	require.NoError(t, err)

	size := st.Size()
	const recordSize = int64(len(storj.PieceID{})) + 8
	require.Equal(t, recordSize, size)
}
