// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecemigrate

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/satstore"
)

func TestDuplicates(t *testing.T) {
	t.Parallel()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	dir, err := filestore.NewDir(log, t.TempDir())
	require.NoError(t, err)

	blobs := filestore.New(log, dir, filestore.DefaultConfig)
	defer ctx.Check(blobs.Close)

	fw := pieces.NewFileWalker(log, blobs, nil, nil, nil)

	bfm, err := retain.NewBloomFilterManager(t.TempDir(), 0)
	require.NoError(t, err)

	rtm := retain.NewRestoreTimeManager(t.TempDir())

	old := pieces.NewStore(log, fw, nil, blobs, nil, nil, pieces.DefaultConfig)
	new := piecestore.NewHashStoreBackend(t.TempDir(), bfm, rtm, log)

	config := Config{
		Interval: 100 * time.Millisecond,
		Delay:    time.Millisecond,
		Jitter:   true,
	}

	chore := NewChore(log, config, satstore.NewSatelliteStore(t.TempDir(), "migrate_chore"), old, new)
	group := errgroup.Group{}
	group.Go(func() error { return chore.Run(ctx) })
	defer ctx.Check(group.Wait)
	defer ctx.Check(chore.Close)

	sats1 := randomSatsPieces(1, 3)
	writeSatsPieces(ctx, t, old, sats1)
	sats2 := randomSatsPieces(2, 6)
	writeSatsPieces(ctx, t, old, sats2)

	setMigrateActive(chore, sats1)
	setMigrateActive(chore, sats2)

	waitUntilMigrationFinished(ctx, t, old, sats1)
	waitUntilMigrationFinished(ctx, t, old, sats2)

	// simulate that the delete has failed
	writeSatsPieces(ctx, t, old, sats1)

	waitUntilMigrationFinished(ctx, t, old, sats1)
	waitUntilMigrationFinished(ctx, t, old, sats2)
}

func TestChoreWithPassiveMigrationOnly(t *testing.T) {
	t.Parallel()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	dir, err := filestore.NewDir(log, t.TempDir())
	require.NoError(t, err)

	blobs := filestore.New(log, dir, filestore.DefaultConfig)
	defer ctx.Check(blobs.Close)

	fw := pieces.NewFileWalker(log, blobs, nil, nil, nil)

	bfm, err := retain.NewBloomFilterManager(t.TempDir(), 0)
	require.NoError(t, err)

	rtm := retain.NewRestoreTimeManager(t.TempDir())

	old := pieces.NewStore(log, fw, nil, blobs, nil, nil, pieces.DefaultConfig)
	new := piecestore.NewHashStoreBackend(t.TempDir(), bfm, rtm, log)

	satellites1 := randomSatsPieces(2, 100)
	writeSatsPieces(ctx, t, old, satellites1)
	satellites2 := randomSatsPieces(2, 100)
	writeSatsPieces(ctx, t, old, satellites2)
	satellites3 := randomSatsPieces(2, 100)
	writeSatsPieces(ctx, t, old, satellites3)

	config := Config{
		BufferSize:        400,
		Interval:          100 * time.Millisecond,
		MigrateRegardless: true,
	}

	satStoreDir, satStoreExt := t.TempDir(), "migrate_chore"

	for i, sat := range maps.Keys(satellites1) {
		var v string
		if i%2 == 0 {
			v = "false"
		} else {
			v = "blabl"
		}
		require.NoError(t, os.WriteFile(filepath.Join(satStoreDir, sat.String()+"."+satStoreExt), []byte(v), 0644))
	}

	chore := NewChore(log, config, satstore.NewSatelliteStore(satStoreDir, satStoreExt), old, new)
	group := errgroup.Group{}
	group.Go(func() error { return chore.Run(ctx) })
	defer ctx.Check(group.Wait)
	defer ctx.Check(chore.Close)

	for sat := range satellites2 {
		chore.SetMigrate(sat, false, false) // explicitly excluded
	}

	for sat, pieces := range satellites2 {
		for _, p := range pieces {
			chore.TryMigrateOne(sat, p.id)
		}
	}
	for sat, pieces := range satellites3 {
		for _, p := range pieces {
			chore.TryMigrateOne(sat, p.id)
		}
	}

	waitUntilMigrationFinished(ctx, t, old, satellites2)
	waitUntilMigrationFinished(ctx, t, old, satellites3)

	// migration complete! let's check if the new backend contains what
	// we migrated to it:
	for sat, pieces := range satellites2 {
		for _, p := range pieces {
			readFromBackend(ctx, t, new, sat, p)
		}
	}
	for sat, pieces := range satellites3 {
		for _, p := range pieces {
			readFromBackend(ctx, t, new, sat, p)
		}
	}
	for sat, pieces := range satellites2 {
		for _, p := range pieces {
			require.False(t, existsInStore(ctx, t, old, sat, p.id))
		}
	}
	for sat, pieces := range satellites3 {
		for _, p := range pieces {
			require.False(t, existsInStore(ctx, t, old, sat, p.id))
		}
	}
	// check that what we didn't want to migrate is still in place:
	for sat, pieces := range satellites1 {
		for _, p := range pieces {
			require.False(t, existsInBackend(ctx, t, new, sat, p.id))
		}
	}
	for sat, pieces := range satellites1 {
		for _, p := range pieces {
			readFromStore(ctx, t, old, sat, p)
		}
	}
}

func TestChoreActiveWithPassiveMigration(t *testing.T) {
	t.Parallel()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	defer ctx.Check(log.Sync)

	dir, err := filestore.NewDir(log, t.TempDir())
	require.NoError(t, err)

	blobs := filestore.New(log, dir, filestore.DefaultConfig)
	defer ctx.Check(blobs.Close)

	fw := pieces.NewFileWalker(log, blobs, nil, nil, nil)

	bfm, err := retain.NewBloomFilterManager(t.TempDir(), 0)
	require.NoError(t, err)

	rtm := retain.NewRestoreTimeManager(t.TempDir())

	old := pieces.NewStore(log, fw, nil, blobs, nil, nil, pieces.DefaultConfig)
	new := piecestore.NewHashStoreBackend(t.TempDir(), bfm, rtm, log)

	migratedSatellites := randomSatsPieces(3, 1000)
	migratedSatellitesMu := sync.Mutex{}
	writeSatsPieces(ctx, t, old, migratedSatellites)

	excludedSatellites1 := randomSatsPieces(1, 1000)
	writeSatsPieces(ctx, t, old, excludedSatellites1)
	excludedSatellites2 := randomSatsPieces(1, 1000)
	writeSatsPieces(ctx, t, old, excludedSatellites2)
	excludedSatellites3 := randomSatsPieces(1, 1000)
	writeSatsPieces(ctx, t, old, excludedSatellites3)

	config := Config{
		BufferSize: 1,
		Interval:   100 * time.Millisecond,
	}

	satStoreDir, satStoreExt := t.TempDir(), "migrate_chore"

	for sat := range migratedSatellites {
		require.NoError(t, os.WriteFile(filepath.Join(satStoreDir, sat.String()+"."+satStoreExt), []byte("true\n"), 0644))
	}

	chore := NewChore(log, config, satstore.NewSatelliteStore(satStoreDir, satStoreExt), old, new)
	group := errgroup.Group{}
	group.Go(func() error { return chore.Run(ctx) })
	defer ctx.Check(group.Wait)
	defer ctx.Check(chore.Close)

	passiveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	group.Go(func() error { // mimick passive migration
		for {
			select {
			case <-passiveCtx.Done():
				return nil
			default:
				for sat, pieces := range excludedSatellites1 {
					max := testrand.Intn(3)
					for i, p := range pieces {
						if i == max {
							break
						}
						chore.TryMigrateOne(sat, p.id)
						time.Sleep(time.Duration(testrand.Int63n(10)) * time.Millisecond)
					}
				}
				for sat, pieces := range excludedSatellites2 {
					max := testrand.Intn(5)
					for i, p := range pieces {
						if i == max {
							break
						}
						chore.TryMigrateOne(sat, p.id)
						time.Sleep(time.Duration(testrand.Int63n(20)) * time.Millisecond)
					}
				}
				migratedSatellitesMu.Lock()
				for sat, pieces := range migratedSatellites {
					max := testrand.Intn(8)
					for i, p := range pieces {
						if i == max {
							break
						}
						chore.TryMigrateOne(sat, p.id)
						time.Sleep(time.Duration(testrand.Int63n(30)) * time.Millisecond)
					}
				}
				migratedSatellitesMu.Unlock()
				for sat, pieces := range excludedSatellites3 {
					max := testrand.Intn(13)
					for i, p := range pieces {
						if i == max {
							break
						}
						chore.TryMigrateOne(sat, p.id)
						time.Sleep(time.Duration(testrand.Int63n(40)) * time.Millisecond)
					}
				}
			}
		}
	})

	for sat := range excludedSatellites1 { // explicitly excluded
		chore.SetMigrate(sat, false, true)
	}

	waitUntilMigrationFinished(ctx, t, old, migratedSatellites)

	// excludedSatellites3 are no longer excluded:
	for sat, pieces := range excludedSatellites3 {
		chore.SetMigrate(sat, true, true)
		migratedSatellitesMu.Lock()
		migratedSatellites[sat] = pieces
		migratedSatellitesMu.Unlock()
	}

	waitUntilMigrationFinished(ctx, t, old, migratedSatellites)

	// migration complete! let's check if the new backend contains what
	// we migrated to it:
	for sat, pieces := range migratedSatellites {
		for _, p := range pieces {
			readFromBackend(ctx, t, new, sat, p)
		}
	}
	for sat, pieces := range migratedSatellites {
		for _, p := range pieces {
			require.False(t, existsInStore(ctx, t, old, sat, p.id))
		}
	}
	// check that what we didn't want to migrate is still in place:
	for sat, pieces := range excludedSatellites1 {
		for _, p := range pieces {
			require.False(t, existsInBackend(ctx, t, new, sat, p.id))
		}
	}
	for sat, pieces := range excludedSatellites1 {
		for _, p := range pieces {
			readFromStore(ctx, t, old, sat, p)
		}
	}
	for sat, pieces := range excludedSatellites2 {
		for _, p := range pieces {
			require.False(t, existsInBackend(ctx, t, new, sat, p.id))
		}
	}
	for sat, pieces := range excludedSatellites2 {
		for _, p := range pieces {
			readFromStore(ctx, t, old, sat, p)
		}
	}
}

// TODO(artur): there's a lot of duplication among the helper functions.
// Making sure that OldPieceBackend implements PieceMigrateBackend would
// allow getting rid of that.

type pieceToCheck struct {
	sat      storj.NodeID
	id       storj.PieceID
	content  []byte
	hashAlgo pb.PieceHashAlgorithm
	hash     []byte
}

func randomSatsPieces(n, nPieces int) map[storj.NodeID][]*pieceToCheck {
	ret := make(map[storj.NodeID][]*pieceToCheck)

	for i := 0; i < n; i++ {
		id := testrand.NodeID()

		var pieces []*pieceToCheck
		for j := 0; j < nPieces; j++ {
			pieces = append(pieces, &pieceToCheck{
				sat:      id,
				id:       testrand.PieceID(),
				content:  testrand.Bytes(memory.Size(testrand.Intn(10)) * memory.KB),
				hashAlgo: pb.PieceHashAlgorithm_SHA256,
			})
		}

		ret[id] = pieces
	}

	return ret
}

func writeSatsPieces(ctx context.Context, t *testing.T, store *pieces.Store, satsPieces map[storj.NodeID][]*pieceToCheck) {
	for sat, pieces := range satsPieces {
		for _, p := range pieces {
			writeToStore(ctx, t, store, sat, p)
		}
	}
}

func writeToStore(ctx context.Context, t *testing.T, store *pieces.Store, sat storj.NodeID, piece *pieceToCheck) {
	w, err := store.Writer(ctx, sat, piece.id, piece.hashAlgo)
	require.NoError(t, err)
	defer func() { require.NoError(t, w.Cancel(ctx)) }()

	n, err := sync2.Copy(ctx, w, bytes.NewReader(piece.content))
	require.NoError(t, err)

	require.Equal(t, len(piece.content), int(n))
	require.Equal(t, len(piece.content), int(w.Size()))

	piece.hash = w.Hash()

	require.NoError(t, w.Commit(ctx, &pb.PieceHeader{Hash: w.Hash()}))
}

func readFromStore(ctx context.Context, t *testing.T, store *pieces.Store, sat storj.NodeID, piece *pieceToCheck) {
	r, err := store.Reader(ctx, sat, piece.id)
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Close()) }()

	hdr, err := r.GetPieceHeader()
	require.NoError(t, err)
	require.Equal(t, piece.hashAlgo, hdr.HashAlgorithm)
	require.Equal(t, piece.hash, hdr.Hash)

	b := bytes.NewBuffer(nil)

	n, err := sync2.Copy(ctx, b, r)
	require.NoError(t, err)

	require.Equal(t, len(piece.content), int(n))
	require.Equal(t, len(piece.content), int(r.Size()))

	require.Equal(t, piece.content, b.Bytes())
}

func readFromBackend(ctx context.Context, t *testing.T, backend piecestore.PieceBackend, sat storj.NodeID, piece *pieceToCheck) {
	r, err := backend.Reader(ctx, sat, piece.id)
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Close()) }()

	hdr, err := r.GetPieceHeader()
	require.NoError(t, err)
	require.Equal(t, piece.hashAlgo, hdr.HashAlgorithm)
	require.Equal(t, piece.hash, hdr.Hash)

	b := bytes.NewBuffer(nil)

	n, err := sync2.Copy(ctx, b, r)
	require.NoError(t, err)

	require.Equal(t, len(piece.content), int(n))
	require.Equal(t, len(piece.content), int(r.Size()))

	require.Equal(t, piece.content, b.Bytes())
}

func existsInStore(ctx context.Context, t *testing.T, store *pieces.Store, sat storj.NodeID, piece storj.PieceID) bool {
	r, err := store.Reader(ctx, sat, piece)
	if err != nil {
		if errs.Is(err, fs.ErrNotExist) {
			return false
		}
		require.NoError(t, err)
	}
	defer func() { require.NoError(t, r.Close()) }()
	return true
}

func existsInBackend(ctx context.Context, t *testing.T, backend piecestore.PieceBackend, sat storj.NodeID, piece storj.PieceID) bool {
	r, err := backend.Reader(ctx, sat, piece)
	if err != nil {
		if errs.Is(err, fs.ErrNotExist) {
			return false
		}
		require.NoError(t, err)
	}
	defer func() { require.NoError(t, r.Close()) }()
	return true
}

func waitUntilMigrationFinished(ctx context.Context, t *testing.T, store *pieces.Store, satsPieces map[storj.NodeID][]*pieceToCheck) {
	for {
		var count int
		for sat := range satsPieces {
			var c int
			require.NoError(t, store.WalkSatellitePieces(ctx, sat, func(spa pieces.StoredPieceAccess) error {
				c++
				return nil
			}))
			t.Logf("%d left to migrate for %s", c, sat)
			count += c
		}
		if count == 0 {
			return
		}
		time.Sleep(time.Second)
	}
}

func setMigrateActive(chore *Chore, satsPieces map[storj.NodeID][]*pieceToCheck) {
	for sat := range satsPieces {
		chore.SetMigrate(sat, true, true)
	}
}
