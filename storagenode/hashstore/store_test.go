// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"

	"storj.io/storj/storagenode/hashstore/platform"
)

func TestStore_BasicOperation(t *testing.T) {
	forAllTables(t, func(t *testing.T, cfg Config) {
		t.Run("sync=false", func(t *testing.T) {
			cfg.Store.SyncWrites = false
			testStore_BasicOperation(t, cfg)
		})
		t.Run("sync=true", func(t *testing.T) {
			cfg.Store.SyncWrites = true
			testStore_BasicOperation(t, cfg)
		})
	})
}
func testStore_BasicOperation(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// ensure stats works before any keys are added.
	stats := s.Stats()
	assert.Equal(t, stats.Table.NumSet, 0)
	assert.Equal(t, stats.Table.LenSet, 0)
	assert.Equal(t, stats.Table.AvgSet, 0.)

	var keys []Key

	// write a bunch of keys and compact a couple of times.
	for i := 0; i < 4; i++ {
		for j := 0; j < 1024; j++ {
			key := s.AssertCreate()
			keys = append(keys, key)
			s.AssertRead(key)
		}
		s.AssertCompact(nil, time.Time{})
	}

	// ensure we can still read all of the keys even after compaction.
	for _, key := range keys {
		s.AssertRead(key)
	}

	// ensure the stats look like what we expect.
	stats = s.Stats()
	t.Logf("%+v", stats)
	assert.Equal(t, stats.Table.NumSet, 4*1024)
	assert.Equal(t, stats.Table.LenSet, uint64(len(Key{})+RecordSize)*stats.Table.NumSet)
	assert.Equal(t, stats.Table.AvgSet, float64(len(Key{})+RecordSize))
	assert.Equal(t, stats.Table.LenSet, stats.LenLogs)
	assert.Equal(t, stats.Compactions, 4)

	// reopen the store and ensure we can still read all of the keys.
	s.AssertReopen()
	for _, key := range keys {
		s.AssertRead(key)
	}

	// create, read, and compact should fail after close.
	s.Close()

	_, err := s.Read(ctx, newKey())
	assert.Error(t, err)

	_, err = s.Create(ctx, newKey(), time.Time{})
	assert.Error(t, err)

	assert.Error(t, s.Compact(ctx, nil, time.Time{}))
}

func TestStore_TrashStats(t *testing.T) {
	forAllTables(t, testStore_TrashStats)
}
func testStore_TrashStats(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	s.AssertCreate()
	s.AssertCompact(alwaysTrash, time.Time{})

	stats := s.Stats()
	assert.Equal(t, stats.Table.NumTrash, 1)
	assert.Equal(t, stats.Table.LenTrash, 96)
	assert.Equal(t, stats.Table.AvgTrash, 96.)
	assert.Equal(t, stats.TrashPercent, 1.)
}

func TestStore_FileLocking(t *testing.T) {
	forAllTables(t, testStore_FileLocking)
}
func testStore_FileLocking(t *testing.T, cfg Config) {
	if !platform.FlockSupported {
		t.Skip("flock not supported on this platform")
	}

	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// flock should stop a second store from being created with the same hashdir.
	_, err := NewStore(ctx, cfg, s.logsPath, "", nil)
	assert.Error(t, err)

	// it should still be locked even after compact makes a new hashtbl file.
	s.AssertCompact(nil, time.Time{})
	_, err = NewStore(ctx, cfg, s.logsPath, "", nil)
	assert.Error(t, err)
}

func TestStore_CreateSameKeyErrors(t *testing.T) {
	forAllTables(t, testStore_CreateSameKeyErrors)
}
func testStore_CreateSameKeyErrors(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// attempting to make the same entry fails on the Close call.
	wr, err := s.Create(t.Context(), key, time.Time{})
	assert.NoError(t, err)
	assert.Error(t, wr.Close())
}

func TestStore_ReadFromCompactedFile(t *testing.T) {
	forAllTables(t, testStore_ReadFromCompactedFile)
}
func testStore_ReadFromCompactedFile(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// add some already expired entries to the store so the log file is compacted.
	for i := 0; i < 100; i++ {
		s.AssertCreate(WithTTL(time.Unix(1, 0)))
	}

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// grab the record for the key so we can compare it to the record after compaction.
	before, ok, err := s.tbl.Lookup(ctx, key)
	assert.NoError(t, err)
	assert.True(t, ok)

	// grab a reader for the key and hold on to it through compaction.
	r, err := s.Read(t.Context(), key)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	defer r.Release()

	// compact the store so that it is flagged as trash.
	s.AssertCompact(alwaysTrash, time.Time{})

	// ensure that the log file for the record changed and the original log file was compacted.
	after, ok, err := s.tbl.Lookup(ctx, key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.That(t, before.Log < after.Log)

	// move to the future so that compaction deletes the record.
	s.today += uint32(s.cfg.Compaction.ExpiresDays) + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(alwaysTrash, time.Time{})

	// we should be able to read the data still because the open handle should retain a reference to
	// the log file.
	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.NoError(t, r.Close())
	assert.Equal(t, data, key[:])

	// grab a new reader for the key. it should be deleted.
	s.AssertNotExist(key)
}

func TestStore_CompactionEventuallyDeletes(t *testing.T) {
	forAllTables(t, testStore_CompactionEventuallyDeletes)
}
func testStore_CompactionEventuallyDeletes(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// compact a bunch of times, every day incrementing by one. we need to do two extra days because
	// the first compaction flags it to be deleted after ExpiresDays, we then need to wait that many
	// days, and then the next compaction will actually delete it.
	for i := uint32(0); i < 1+uint32(s.cfg.Compaction.ExpiresDays)+1; i++ {
		s.AssertCompact(alwaysTrash, time.Time{})
		s.today++
	}

	// grab a reader for the key. it should be deleted.
	s.AssertNotExist(key)
}

func TestStore_DeleteTrashImmediately(t *testing.T) {
	forAllTables(t, testStore_DeleteTrashImmediately)
}
func testStore_DeleteTrashImmediately(t *testing.T, cfg Config) {
	cfg.Compaction.DeleteTrashImmediately = true

	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry that does not expire.
	key := s.AssertCreate()

	// compact once. it should be deleted right away.
	s.AssertCompact(alwaysTrash, time.Time{})
	s.AssertNotExist(key)
}

func TestStore_DeleteTrashImmediately_ExistingTrash(t *testing.T) {
	forAllTables(t, testStore_DeleteTrashImmediately_ExistingTrash)
}

func testStore_DeleteTrashImmediately_ExistingTrash(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry that does not expire.
	key := s.AssertCreate()

	// compact once. it should still exist.
	s.AssertCompact(alwaysTrash, time.Time{})
	s.AssertExist(key)

	// go forward in time but not enough to expire the key and compact again. it should still exist.
	s.today += uint32(s.cfg.Compaction.ExpiresDays) / 2
	s.AssertCompact(alwaysTrash, time.Time{})
	s.AssertExist(key)

	s.cfg.Compaction.DeleteTrashImmediately = true
	s.AssertReopen()

	// now compaction should delete the key. it should not exist.
	s.AssertCompact(alwaysTrash, time.Time{})
	s.AssertNotExist(key)
}

func TestStore_CompactionRespectsRestoreTime(t *testing.T) {
	forAllTables(t, testStore_CompactionRespectsRestoreTime)
}
func testStore_CompactionRespectsRestoreTime(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// flag the key as trash.
	s.AssertCompact(alwaysTrash, time.Time{})

	// assume a restore call came in today.
	restore := DateToTime(s.today)

	// compact again far enough ahead to ensure it would be deleted if not for restore.
	s.today += uint32(s.cfg.Compaction.ExpiresDays) + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, restore)

	// grab a reader for the key. it should still exist.
	s.AssertRead(key)
}

func TestReader_ReviveOnNonTrash(t *testing.T) {
	forAllTables(t, testReader_ReviveOnNonTrash)
}
func testReader_ReviveOnNonTrash(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// Create a key that is not trashed
	key := s.AssertCreate()

	// Get a reader for the key
	r, err := s.Read(ctx, key)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	defer r.Release()

	// Verify that the reader is not in trash state
	assert.False(t, r.Trash())

	// Calling Revive on a non-trashed Reader should be safe and return nil
	err = r.Revive(ctx)
	assert.NoError(t, err)

	// Verify that the key still exists and is not trashed
	s.AssertRead(key, AssertTrash(false))
}

func TestStore_TTL(t *testing.T) {
	forAllTables(t, testStore_TTL)
}
func testStore_TTL(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry to the store that is already expired.
	key := s.AssertCreate(WithTTL(time.Now()))

	// ensure the stats have it in the ttl section.
	stats := s.Stats()
	assert.Equal(t, stats.NumLogs, stats.NumLogsTTL)
	assert.Equal(t, stats.LenLogs, stats.LenLogsTTL)

	// compact the store so that the expired key is deleted.
	s.today += 3 // 3 just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})

	// grab a reader for the key. it should be deleted.
	s.AssertNotExist(key)
}

func TestStore_CompactionWithTTLTakesShorterTime(t *testing.T) {
	forAllTables(t, testStore_CompactionWithTTLTakesShorterTime)
}
func testStore_CompactionWithTTLTakesShorterTime(t *testing.T, cfg Config) {
	t.Run("CompactionShorter", func(t *testing.T) {
		s := newTestStore(t, cfg)
		defer s.Close()

		// add an entry to the store that will expire way in the future.
		key := s.AssertCreate(WithTTL(time.Now().AddDate(0, 0, 10*int(s.cfg.Compaction.ExpiresDays))))

		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// bump time to the minimum necessary to expire the key.
		s.today += uint32(s.cfg.Compaction.ExpiresDays) + 1 // 1 more just in case the test is running near midnight.
		s.AssertCompact(nil, time.Time{})

		// the key should not exist.
		s.AssertNotExist(key)
	})

	t.Run("TTLShorter", func(t *testing.T) {
		s := newTestStore(t, cfg)
		defer s.Close()

		// add an entry to the store that is already expired.
		key := s.AssertCreate(WithTTL(time.Unix(1, 0)))

		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// bump time to the minimum necessary to expire the key.
		s.today += 3 // 3 just in case the test is running near midnight.
		s.AssertCompact(nil, time.Time{})

		// the key should not exist.
		s.AssertNotExist(key)
	})
}

func TestStore_CompactLogFile(t *testing.T) {
	t.Run("ignoreRewriteIndex=false", func(t *testing.T) {
		forAllTables(t, testStore_CompactLogFile)
	})
	t.Run("ignoreRewriteIndex=true", func(t *testing.T) {
		test_Store_IgnoreRewrittenIndex = true
		defer func() { test_Store_IgnoreRewrittenIndex = false }()
		forAllTables(t, testStore_CompactLogFile)
	})
}

func testStore_CompactLogFile(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// add some entries to the store that we will expire with a compaction. this is to ensure they
	// are added to the same log file as the live keys we add next.
	var expired []Key
	for i := 0; i < 10; i++ {
		expired = append(expired, s.AssertCreate())
	}
	s.AssertCompact(alwaysTrash, time.Time{})

	// add some entries to the store that are not expired. keep track of their records in the
	// hashtbl so that we can ensure they are in a new log file after compaction.
	var live []Key
	var recs []Record
	for i := 0; i < 10; i++ {
		key := s.AssertCreate()
		live = append(live, key)

		rec, ok, err := s.tbl.Lookup(ctx, key)
		assert.NoError(t, err)
		assert.True(t, ok)
		recs = append(recs, rec)
	}

	// compact the store so that the expired keys are deleted.
	s.today += uint32(s.cfg.Compaction.ExpiresDays) + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})

	// all the expired keys should be deleted.
	for _, key := range expired {
		s.AssertNotExist(key)
	}

	// all the live keys should still exist, but be in a new log file.
	for n, key := range live {
		s.AssertRead(key)
		exp := recs[n]

		got, ok, err := s.tbl.Lookup(ctx, key)
		assert.NoError(t, err)
		assert.True(t, ok)

		assert.False(t, RecordsEqualish(got, exp)) // ensure they are different.
		got.Log, exp.Log = 0, 0
		got.Offset, exp.Offset = 0, 0
		assert.True(t, RecordsEqualish(got, exp)) // but only in their log location.
	}
}

func TestStore_ClumpObjectsByTTL(t *testing.T) {
	forAllTables(t, testStore_ClumpObjectsByTTL)
}
func testStore_ClumpObjectsByTTL(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	check := func(key Key, log int) {
		rec, ok, err := s.tbl.Lookup(ctx, key)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, rec.Log, log)
	}

	now := time.Now()
	s.today = TimeToDateDown(now)

	// run twice with a reopen to ensure the ttl is persisted.
	for runs := 0; runs < 2; runs++ {
		// add some entries to the store with different ttls and ensure they get different log files.
		for i := 0; i < 100; i++ {
			key := s.AssertCreate(WithTTL(now.Add(24 * time.Hour * time.Duration(i))))
			check(key, i+1)
		}

		// but if the ttl is too far ahead, it should get the same log as the no-ttl entries.
		key := s.AssertCreate()
		check(key, 101)
		key = s.AssertCreate(WithTTL(now.Add(24 * time.Hour * 100)))
		check(key, 101)

		s.AssertReopen()
	}
}

func TestStore_WriteCancel(t *testing.T) {
	forAllTables(t, testStore_WriteCancel)
}
func testStore_WriteCancel(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// grab a writer for the key and cancel it a bunch.
	var keys []Key
	for i := 0; i < 1000; i++ {
		key := newKey()
		keys = append(keys, key)

		wr, err := s.Create(t.Context(), key, time.Time{})
		assert.NoError(t, err)

		_, err = wr.Write(make([]byte, 1024))
		assert.NoError(t, err)

		// cancel and close should be idempotent.
		for j := 0; j < 5; j++ {
			wr.Cancel()
			assert.Error(t, wr.Close()) // close after cancel is an error
		}

		// writing after either should return an error.
		_, err = wr.Write(nil)
		assert.Error(t, err)
	}

	// none of the keys should be present.
	for _, key := range keys {
		s.AssertNotExist(key)
	}

	// there should be one log file and it should be empty, but the file did have bytes written so
	// it should be that long.
	var looped flag
	assert.NoError(t, s.lfs.Range(func(_ uint64, lf *logFile) (bool, error) {
		assert.False(t, looped.set())
		assert.Equal(t, lf.size.Load(), 0)
		size, err := fileSize(lf.fh)
		assert.NoError(t, err)
		assert.Equal(t, size, 1024)
		return true, nil
	}))
}

func TestStore_ReadRevivesTrash(t *testing.T) {
	forAllTables(t, testStore_ReadRevivesTrash)
}
func testStore_ReadRevivesTrash(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	for i := uint32(0); i < 5*uint32(s.cfg.Compaction.ExpiresDays); i++ {
		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// grab a reader for the key. it should still exist.
		s.AssertRead(key, WithRevive(true))

		// move on to the next day.
		s.today++
	}

	// ensure the Trash flag is set.
	s.AssertCompact(alwaysTrash, time.Time{})
	r, err := s.Read(t.Context(), key)
	defer r.Release()
	assert.NoError(t, err)
	assert.True(t, r.Trash())
}

func TestStore_LogFilesFull(t *testing.T) {
	forAllTables(t, testStore_LogFilesFull)
}
func testStore_LogFilesFull(t *testing.T, cfg Config) {
	cfg.Compaction.MaxLogSize = 256

	s := newTestStore(t, cfg)
	defer s.Close()

	// Create enough data to fill multiple log files
	var keys []Key
	for i := 0; i < 100; i++ {
		key := s.AssertCreate()
		keys = append(keys, key)
	}

	// Verify all pieces can still be read
	for _, key := range keys {
		s.AssertRead(key)
	}

	// Create additional data after reads
	for i := 0; i < 50; i++ {
		key := s.AssertCreate()
		keys = append(keys, key)
	}

	// Verify all pieces can still be read
	for _, key := range keys {
		s.AssertRead(key)
	}

	// Check stats to make sure we've used multiple log files
	stats := s.Stats()
	assert.That(t, stats.NumLogs > 1)
}

func TestStore_MergeRecordsWhenCompactingWithLostPage(t *testing.T) {
	ctx := t.Context()
	s := newTestStore(t, CreateDefaultConfig(TableKind_HashTbl, false))
	defer s.Close()

	// create two keys that collide at the end of the first page.
	k0 := newKeyAt(s.tbl.(*HashTbl), 0, recordsPerPage-1, 0)
	k1 := newKeyAt(s.tbl.(*HashTbl), 0, recordsPerPage-1, 1)

	// write k0 and k1 to the store.
	s.AssertCreate(WithKey(k0))
	s.AssertCreate(WithKey(k1))

	// create a large key in the third page so that the log file is kept alive.
	kl := newKeyAt(s.tbl.(*HashTbl), 2, 0, 0)

	s.AssertCreate(WithKey(kl), WithDataSize(10*1024))

	// compact the store flagging k1 as trash.
	assert.NoError(t, s.Compact(ctx, func(_ context.Context, key Key, _ time.Time) bool {
		return key == k1
	}, time.Time{}))

	// clear out the first page so that any updates to k1 don't overwrite the existing entry for k1.
	_, err := s.tbl.Handle().WriteAt(make([]byte, pageSize), tbl_headerSize) // offset=headerSize to skip the header page
	assert.NoError(t, err)

	// reading k1 will cause it to revive, adding the duplicate entry for k1.
	s.AssertRead(k1, WithRevive(true))

	// ensure the only entries in the table are duplicate k1 entries and kl.
	keys := []Key{k1, k1, kl}
	assert.NoError(t, s.tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
		assert.Equal(t, rec.Key, keys[0])
		keys = keys[1:]
		return true, nil
	}))
	assert.Equal(t, len(keys), 0)

	// when we compact, it should take the later expiration for k1 so it will never delete it.
	s.AssertCompact(nil, time.Time{})

	// bump the day so that if it were to delete k1, it would have.
	s.today += uint32(s.cfg.Compaction.ExpiresDays) + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})

	// k1 should still be reachable.
	s.AssertRead(k1)
}

func TestStore_ReviveDuringCompaction(t *testing.T) {
	forAllTables(t, testStore_ReviveDuringCompaction)
}
func testStore_ReviveDuringCompaction(t *testing.T, cfg Config) {
	run := func(t *testing.T, future uint32) {
		ctx := t.Context()
		s := newTestStore(t, cfg)
		defer s.Close()

		// insert the key we'll be reviving.
		key := s.AssertCreate()

		// compact the store so that the key is trashed.
		s.AssertCompact(alwaysTrash, time.Time{})

		// insert a 2nd key that we will have to call the trash callback on so we can control the
		// progress of the compaction.
		s.AssertCreate()

		// potentially go into the future so that the key is maybe deleted.
		s.today += future

		// begin a compaction in the background that we can control when it proceeds with the trash
		// callback.
		activity := make(chan bool)
		errCh := make(chan error)

		go func() {
			errCh <- s.Compact(ctx,
				func(ctx context.Context, key Key, created time.Time) bool {
					for range activity { // wait until we are closed to continue.
					}
					return true // we return true so that compaction doesn't exit early.
				}, time.Time{})
		}()

		// wait until the compaction is asking to trash our 2nd key.
		activity <- false

		// start a goroutine that waits for this test to be blocked trying to grab a writer for
		// reviving the key, then allows compaction to continue.
		go func() {
			waitForGoroutine(
				"testStore_ReviveDuringCompaction",
				"(*testStore).AssertRead",
				"(*Store).reviveRecord",
				"(*mutex).Lock",
			)
			// the following AssertRead call is blocked on Create, allow compaction to finish.
			close(activity)
		}()

		// try to read the key which will attempt to revive it while compaction is running.
		s.AssertRead(key, WithRevive(true))

		// compaction should finish without error.
		assert.NoError(t, <-errCh)

		// revive could have done nothing. after compaction is finished we can read again to ensure
		// it was actually revived.
		s.AssertRead(key, AssertTrash(false))
	}

	t.Run("Dead", func(t *testing.T) { run(t, uint32(cfg.Compaction.ExpiresDays)+1) })
	t.Run("Alive", func(t *testing.T) { run(t, 0) })
}

func TestStore_MultipleReviveDuringCompaction(t *testing.T) {
	forAllTables(t, testStore_MultipleReviveDuringCompaction)
}
func testStore_MultipleReviveDuringCompaction(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// insert the keys we'll be reviving.
	key0 := s.AssertCreate()
	key1 := s.AssertCreate()

	// compact the store so that the key is trashed.
	s.AssertCompact(alwaysTrash, time.Time{})

	// add a key that isn't yet trashed so that compaction queries it.
	s.AssertCreate()

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(ctx,
			func(ctx context.Context, key Key, created time.Time) bool {
				for range activity { // wait until we are closed to continue.
				}
				return true // we have to return true so that compaction doesn't exit early.
			}, time.Time{})
	}()

	// wait until compaction is asking to trash a key, so we know it's running.
	activity <- false

	// start a goroutine that waits for 2 stacks to be blocked in reviveRecord.
	go func() {
		waitForGoroutines(2,
			"(*Store).reviveRecord",
			"(*mutex).Lock",
		)
		close(activity)
	}()

	// start functions reading the two keys that will do the revive.
	read := func(k Key) {
		r, err := s.Read(ctx, k)
		_ = r.Revive(ctx)
		_ = r.Close()
		errCh <- err
	}
	go read(key0)
	go read(key1)

	// compaction and the two reads should finish without error.
	for i := 0; i < 3; i++ {
		assert.NoError(t, <-errCh)
	}

	// revive could have done nothing. after compaction is finished we can read again to ensure
	// it was actually revived.
	s.AssertRead(key0, AssertTrash(false))
	s.AssertRead(key1, AssertTrash(false))
}

func TestStore_CloseCancelsCompaction(t *testing.T) {
	forAllTables(t, testStore_CloseCancelsCompaction)
}
func testStore_CloseCancelsCompaction(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// insert some keys for compaction to attempt to process.
	for i := 0; i < 10; i++ {
		s.AssertCreate()
	}

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(t.Context(),
			func(ctx context.Context, key Key, created time.Time) bool {
				for !<-activity { // wait until we are sent true to continue.
				}
				<-ctx.Done() // wait for the context to be canceled.
				return false
			}, time.Time{})
	}()

	// wait until the compaction is asking to trash our key and allow it to proceed to block on the
	// context being canceled.
	activity <- true

	// launch a goroutine that confirms that this test has a Close call blocked in Close then
	// closes the store.
	go func() {
		waitForGoroutine(
			"testStore_CloseCancelsCompaction",
			"(*Writer).Close",
		)
		s.Close()
	}()

	// try to create a key and ensure it fails on Close.
	w, err := s.Create(t.Context(), newKey(), time.Time{})
	assert.NoError(t, err)
	assert.Error(t, w.Close())

	// compaction should have errored.
	assert.Error(t, <-errCh)
}

func TestStore_ContextCancelsClose(t *testing.T) {
	forAllTables(t, testStore_ContextCancelsClose)
}
func testStore_ContextCancelsClose(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// insert a key for compaction to attempt to process.
	s.AssertCreate()

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(t.Context(),
			func(ctx context.Context, key Key, created time.Time) bool {
				for !<-activity { // wait until we are sent true to continue.
				}
				return false
			}, time.Time{})
	}()

	// wait until the compaction is asking to trash our key and allow it to proceed to block on the
	// context being canceled.
	activity <- false

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// launch a goroutine that confirms that this test has a Close call blocked in Close then
	// cancels the context.
	go func() {
		waitForGoroutine(
			"testStore_ContextCancelsClose",
			"(*Writer).Close",
		)
		cancel()
	}()

	// try to create a key and ensure it fails during Close.
	w, err := s.Create(ctx, newKey(), time.Time{})
	assert.NoError(t, err)
	assert.Error(t, w.Close())

	// allow compaction to finish.
	activity <- true

	// compaction should not have errored.
	assert.NoError(t, <-errCh)
}

func TestStore_LogContainsDataToReconstruct(t *testing.T) {
	forAllTables(t, testStore_LogContainsDataToReconstruct)
}
func testStore_LogContainsDataToReconstruct(t *testing.T, cfg Config) {
	const parallelism = 4

	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// write a bunch of keys in parallel and try to spread them across log files. we also write
	// random sizes so stress the reading code some.
	done := make(chan error, parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			done <- func() error {
				rng := mwc.Rand()
				for j := 0; j < 128; j++ {
					buf := make([]byte, rng.Intn(1024))
					_, _ = rng.Read(buf)

					if w, err := s.Create(ctx, newKey(), time.Time{}); err != nil {
						return err
					} else if _, err := w.Write(buf); err != nil {
						return err
					} else if err := w.Close(); err != nil {
						return err
					}
				}
				return nil
			}()
		}()
	}
	for i := 0; i < parallelism; i++ {
		assert.NoError(t, <-done)
	}

	// add some garbage data to the end of the log files to ensure that doesn't break anything.
	assert.NoError(t, s.lfs.Range(func(_ uint64, lf *logFile) (bool, error) {
		buf := make([]byte, mwc.Intn(128))
		_, _ = mwc.Rand().Read(buf)
		_, err := lf.fh.Write(buf)
		return true, err
	}))

	// collect all of the records from the log files.
	collectRecords := func(lf *logFile) (recs []Record) {
		readRecord := func(off int64) (rec Record, ok bool) {
			var buf [RecordSize]byte
			_, err := lf.fh.ReadAt(buf[:], off)
			assert.NoError(t, err)
			ok = rec.ReadFrom(&buf)
			return rec, ok
		}

		off, err := fileSize(lf.fh)
		assert.NoError(t, err)
		off -= RecordSize

		for off >= 0 {
			rec, ok := readRecord(off)
			if !ok {
				off--
				continue
			}
			recs = append(recs, rec)
			off = int64(rec.Offset) - RecordSize
		}

		return recs
	}

	var lfRecs []Record
	assert.NoError(t, s.lfs.Range(func(_ uint64, lf *logFile) (bool, error) {
		lfRecs = append(lfRecs, collectRecords(lf)...)
		return true, nil
	}))

	// collect all the records in the hash table.
	var tblRecs []Record
	assert.NoError(t, s.tbl.Range(ctx, func(ctx context.Context, rec Record) (bool, error) {
		tblRecs = append(tblRecs, rec)
		return true, nil
	}))

	// both sets of records should be equal.
	sort.Slice(lfRecs, func(i, j int) bool {
		return string(lfRecs[i].Key[:]) < string(lfRecs[j].Key[:])
	})
	sort.Slice(tblRecs, func(i, j int) bool {
		return string(tblRecs[i].Key[:]) < string(tblRecs[j].Key[:])
	})
	assert.DeepEqual(t, lfRecs, tblRecs)
}

func TestStore_RaceConcurrentWriteAndStats(t *testing.T) {
	forAllTables(t, testStore_RaceConcurrentWriteAndStats)
}
func testStore_RaceConcurrentWriteAndStats(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		for i := 0; i < 1000; i++ {
			_ = s.Stats()
		}
	}()

	for i := 0; i < 1000; i++ {
		s.AssertCreate()
	}
	<-done
}

func TestStore_FailedUpdateDoesntIncreaseLogLength(t *testing.T) {
	forAllTables(t, testStore_FailedUpdateDoesntIncreaseLogLength)
}
func testStore_FailedUpdateDoesntIncreaseLogLength(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	getSize := func() (size uint64) {
		assert.NoError(t, s.lfs.Range(func(_ uint64, lf *logFile) (bool, error) {
			size = lf.size.Load()
			return false, nil
		}))
		return size
	}
	// add a key to the store
	key := s.AssertCreate()

	// get the size of the log file
	size := getSize()
	assert.NotEqual(t, size, 0)

	// try to update the key. it should fail because the hashtbl does not allow updates.
	w, err := s.Create(ctx, key, time.Time{})
	assert.NoError(t, err)
	_, err = w.Write(make([]byte, 500))
	assert.NoError(t, err)
	assert.Error(t, w.Close())

	// the size of the log file should not have changed
	newSize := getSize()
	assert.Equal(t, size, newSize)
}

func TestStore_CompactionMakesForwardProgress(t *testing.T) {
	forAllTables(t, testStore_CompactionMakesForwardProgress)
}
func testStore_CompactionMakesForwardProgress(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// we are testing when compaction is trying to rewrite a log file that
	// contains more used data than the next hashtbl. we'll do this by having
	// a single large (multi-MB) entry and many small but dead entries and
	// triggering compaction.

	large := s.AssertCreate(WithDataSize(10 << 20))
	for i := 0; i < 1<<10; i++ {
		s.AssertCreate(WithDataSize(10 << 10))
	}
	s.AssertCompact(func(ctx context.Context, key Key, created time.Time) bool {
		return key != large
	}, time.Time{})

	// compact the store so that the expired key is deleted.
	s.today += uint32(s.cfg.Compaction.ExpiresDays) + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})
}

func TestStore_CompactionExitsEarlyWhenNoModifications(t *testing.T) {
	forAllTables(t, testStore_CompactionExitsEarlyWhenNoModifications)
}
func testStore_CompactionExitsEarlyWhenNoModifications(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	s.AssertCreate()
	today := s.today

	check := func(lastCompact, created uint32) {
		stats := s.Stats()
		assert.Equal(t, stats.LastCompact, lastCompact)
		assert.Equal(t, stats.Table.Created, created)
	}

	check(0, today)

	s.today++
	s.AssertCompact(alwaysTrash, time.Time{})
	check(today+1, today+1)

	s.today++
	s.AssertCompact(alwaysTrash, time.Time{})
	check(today+2, today+1)
}

func TestStore_FallbackToNonTTLLogFile(t *testing.T) {
	forAllTables(t, testStore_FallbackToNonTTLLogFile)
}
func testStore_FallbackToNonTTLLogFile(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// add a key that goes into a non-ttl log file.
	permKey := s.AssertCreate()

	// create the log file that the ttl key would go into outside of the knowledge of the store so
	// that it fails when trying to create it itself.
	now := time.Now()
	ttl := TimeToDateUp(now)
	id := s.LogFile(permKey) + 1
	dir := filepath.Join(s.logsPath, fmt.Sprintf("%02x", byte(id)))
	assert.NoError(t, os.MkdirAll(dir, 0755))
	name := filepath.Join(dir, createLogName(id, ttl))
	fh, err := os.OpenFile(name, os.O_CREATE, 0)
	assert.NoError(t, err)
	assert.NoError(t, fh.Close())

	// add a key that goes into a ttl log file. it should succeed anyway.
	ttlKey := s.AssertCreate(WithTTL(now))

	// they should be in the same log file.
	assert.Equal(t, s.LogFile(permKey), s.LogFile(ttlKey))
}

func TestStore_TableFull(t *testing.T) {
	forAllTables(t, testStore_TableFull)
}
func testStore_TableFull(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	for {
		w, err := s.Create(ctx, newKey(), time.Time{})
		assert.NoError(t, err)
		if err := w.Close(); err != nil {
			assert.True(t, strings.Contains(err.Error(), "hashtbl full"))
			return
		}
	}
}

func TestStore_StatsWhileCompacting(t *testing.T) {
	forAllTables(t, testStore_StatsWhileCompacting)
}
func testStore_StatsWhileCompacting(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// insert a key for compaction to attempt to process.
	s.AssertCreate()

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(t.Context(),
			func(ctx context.Context, key Key, created time.Time) bool {
				for range activity { // wait until we are closed to continue.
				}
				return false
			}, time.Time{})
	}()

	// wait until the compaction is asking to trash our key.
	activity <- false

	// stats should indicate we're compacting.
	stats := s.Stats()
	assert.That(t, stats.Compacting)
	assert.That(t, stats.Compaction.Elapsed > 0)

	// allow compaction to finish.
	close(activity)

	// compaction should not have errored.
	assert.NoError(t, <-errCh)
}

func TestStore_CompactionRewritesLogsWhenNothingToDo(t *testing.T) {
	forAllTables(t, testStore_CompactionRewritesLogsWhenNothingToDo)
}
func testStore_CompactionRewritesLogsWhenNothingToDo(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	// make a ballast key that stays alive that should prevent the log file from being rewritten
	// under normal conditions and an already expired key so that the log is not fully alive so that
	// it can be considered a candidate regardless.
	ballast := s.AssertCreate(WithDataSize(4096))
	assert.Equal(t, s.LogFile(ballast), 1)

	s.AssertCreate(WithData(nil), WithTTL(time.Unix(1, 0)))

	// on the first compaction the log should be rewritten.
	s.AssertCompact(nil, time.Time{})
	assert.Equal(t, s.LogFile(ballast), 2)

	{
		stats := s.Stats()
		assert.Equal(t, stats.Compactions, 1)
		assert.Equal(t, stats.LogsRewritten, 1)
		assert.Equal(t, stats.DataRewritten, 4096+RecordSize)
	}

	// the log should be fully alive data and so should not be rewritten on subsequent compactions.
	s.AssertCompact(nil, time.Time{})
	assert.Equal(t, s.LogFile(ballast), 2)

	{
		stats := s.Stats()
		assert.Equal(t, stats.Compactions, 2)
		assert.Equal(t, stats.LogsRewritten, 1)
		assert.Equal(t, stats.DataRewritten, 4096+RecordSize)
	}

	// we should still be able to read the ballast still.
	s.AssertRead(ballast, WithDataSize(4096))
}

func TestStore_FlushSemaphore(t *testing.T) {
	cfg := CreateDefaultConfig(TableKind_HashTbl, false)
	cfg.Store.FlushSemaphore = 1

	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	// Channels to coordinate with the goroutines
	acquired := make(chan struct{})
	release := make(chan struct{})
	errCh := make(chan error)

	// Start a goroutine that will hold the flush lock
	go func() {
		// Acquire flush lock explicitly
		err := s.flushMu.RLock(ctx, &s.closed)
		if err != nil {
			errCh <- err
			return
		}

		// Signal we have acquired the lock
		close(acquired)

		// Wait until the test function indicates we should release the lock
		<-release

		// Release the lock
		s.flushMu.RUnlock()

		// Signal we're done without error
		errCh <- nil
	}()

	// Wait for the goroutine to acquire the lock
	<-acquired

	// Start a goroutine that waits for the test to block on the flush semaphore
	go func() {
		waitForGoroutine(
			"(*Writer).Close",
			"(*rwMutex).RLock",
		)
		// Once we detect blocking, signal the first goroutine to release the lock
		close(release)
	}()

	// Create a key and try to close a writer, which should block on the flush semaphore
	key := s.AssertCreate()

	// Wait for the first goroutine to finish without error
	assert.NoError(t, <-errCh)

	// Read back the data to ensure everything worked
	s.AssertRead(key)
}

func TestStore_SwapDifferentBackends(t *testing.T) {
	backends := []TableKind{TableKind_HashTbl, TableKind_MemTbl}
	cfg := CreateDefaultConfig(TableKind_HashTbl, false)
	s := newTestStore(t, cfg)
	defer s.Close()

	var keys []Key
	for i := 0; i < 10; i++ {
		func() {
			s.cfg.TableDefaultKind.Kind = backends[i%len(backends)]

			// reopen the store sometimes to ensure everything loads correctly
			if i%3 == 0 {
				s.AssertReopen()
			}

			// add a new key and compact which should write a new backend.
			keys = append(keys, s.AssertCreate())
			s.AssertCompact(nil, time.Time{})

			// ensure we can still read all the keys.
			for _, key := range keys {
				s.AssertRead(key)
			}
		}()
	}
}

func TestStore_WriteRandomSizes(t *testing.T) {
	forAllTables(t, testStore_WriteRandomSizes)
}
func testStore_WriteRandomSizes(t *testing.T, cfg Config) {
	ctx := t.Context()
	s := newTestStore(t, cfg)
	defer s.Close()

	data := make([]byte, 1024)

	for i := 0; i < 10; i++ {
		key := newKey()
		_, _ = mwc.Rand().Read(data)

		w, err := s.Create(ctx, key, time.Time{})
		assert.NoError(t, err)

		for buf := data; len(buf) > 0; {
			n := mwc.Intn(len(buf) + 1)
			_, err := w.Write(buf[:n])
			assert.NoError(t, err)
			buf = buf[n:]
		}

		assert.Equal(t, w.Size(), len(data))
		assert.NoError(t, w.Close())

		s.AssertRead(key, WithData(data))
	}
}

func TestStore_RewriteMultipleZeroRemovesFullyDeadLogs(t *testing.T) {
	forAllTables(t, testStore_RewriteMultipleZeroRemovesFullyDeadLogs)
}
func testStore_RewriteMultipleZeroRemovesFullyDeadLogs(t *testing.T, cfg Config) {
	cfg.Compaction.RewriteMultiple = 0
	cfg.Compaction.MaxLogSize = 1024

	s := newTestStore(t, cfg)
	defer s.Close()

	// we want to have one log file that is partially dead and one log file that is fully dead. when
	// we compact, it should rewrite the fully dead log file but not the partially dead one.
	alive := s.AssertCreate(WithDataSize(512))
	dead := s.AssertCreate(WithDataSize(512), WithTTL(time.Unix(1, 0)))
	fullyDead := s.AssertCreate(WithDataSize(512), WithTTL(time.Unix(1, 0)))

	assert.Equal(t, s.LogFile(alive), 1)
	assert.Equal(t, s.LogFile(dead), 1)
	assert.Equal(t, s.LogFile(fullyDead), 2)

	// no matter how many times we compact, fully dead should be removed and partially dead should
	// not be rewritten.
	for i := 0; i < 5; i++ {
		s.AssertCompact(nil, time.Time{})

		assert.Equal(t, s.LogFile(alive), 1)
		s.AssertNotExist(dead)
		s.AssertNotExist(fullyDead)

		stats := s.Stats()
		assert.Equal(t, stats.Compactions, i+1)
		assert.Equal(t, stats.LogsRewritten, 1)
		assert.Equal(t, stats.DataRewritten, 0)
	}
}

func TestStore_CompactionCanceledAfterPartialRewrite(t *testing.T) {
	s := newTestStore(t, CreateDefaultConfig(TableKind_HashTbl, false))
	defer s.Close()

	var keys []Key
	keys = append(keys, s.AssertCreate(WithDataSize(512)))
	keys = append(keys, s.AssertCreate(WithDataSize(512)))
	keys = append(keys, s.AssertCreate(WithDataSize(512), WithTTL(time.Unix(1, 0))))
	keys = append(keys, s.AssertCreate(WithDataSize(512), WithTTL(time.Unix(1, 0))))

	activity := make(chan struct{})
	errCh := make(chan error)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		errCh <- s.Compact(ctx,
			func(ctx context.Context, key Key, created time.Time) bool {
				activity <- struct{}{}
				select {
				case <-activity:
				case <-ctx.Done():
				}
				return true // we return true so that compaction doesn't exit early.
			}, time.Time{})
	}()

	// allow the first shouldTrash call to proceed: this is checking if modiciations are necessary.
	<-activity
	activity <- struct{}{}

	// allow the next shouldTrash call to start. after that, the two alive keys have been rewritten.
	// cancel the compaction before it can finish.
	<-activity
	cancel()
	assert.Error(t, <-errCh)

	// add some new data and ensure we can read everything still.
	keys = append(keys, s.AssertCreate(WithDataSize(512)))
	for _, key := range keys {
		s.AssertRead(key, WithDataSize(512))
	}
}

func TestStore_RewriteMultipleLogFilesInOneCompaction(t *testing.T) {
	cfg := CreateDefaultConfig(TableKind_HashTbl, false)
	cfg.Compaction.MaxLogSize = 1024

	s := newTestStore(t, cfg)
	defer s.Close()

	// fill up 10 logs with an empty alive key and large dead key.
	var alive []Key
	var dead []Key
	for i := 0; i < 10; i++ {
		alive = append(alive, s.AssertCreate(WithDataSize(0)))
		dead = append(dead, s.AssertCreate(WithDataSize(1024), WithTTL(time.Unix(1, 0))))
	}

	// ensure each key is in its own log file.
	for i, key := range alive {
		assert.Equal(t, s.LogFile(key), i+1)
	}

	// compact.
	s.AssertCompact(nil, time.Time{})

	// ensure each alive key is in a new log file and they're all in it and that all the dead keys
	// are gone.
	for _, key := range alive {
		assert.Equal(t, s.LogFile(key), 11)
	}
	for _, key := range dead {
		s.AssertNotExist(key)
	}
}

func TestStore_CompactionMakesProgressEvenIfSmallRewriteMultiple(t *testing.T) {
	cfg := CreateDefaultConfig(TableKind_HashTbl, false)
	cfg.Compaction.RewriteMultiple = 1e-10

	s := newTestStore(t, cfg)
	defer s.Close()

	// create a log file with enough dead data to trigger a rewrite but the amount of alive data
	// puts it over the target threshold of compaction.
	alive := s.AssertCreate(WithDataSize(1024))
	aliveLog := s.LogFile(alive)
	dead := s.AssertCreate(WithDataSize(10240), WithTTL(time.Unix(1, 0)))

	// compact.
	s.AssertCompact(nil, time.Time{})

	// the dead key should be removed and the alive key should be in a new log file.
	s.AssertNotExist(dead)
	assert.NotEqual(t, aliveLog, s.LogFile(alive))
}

func TestStore_OpenFailsWithLogFilesButNoTable(t *testing.T) {
	forAllTables(t, testStore_OpenFailsWithLogFilesButNoTable)
}
func testStore_OpenFailsWithLogFilesButNoTable(t *testing.T, cfg Config) {
	s := newTestStore(t, cfg)
	defer s.Close()

	s.AssertCreate()
	s.Close()

	assert.NoError(t, os.Remove(filepath.Join(s.tablePath, "hashtbl")))

	_, err := NewStore(t.Context(), cfg, s.logsPath, s.tablePath, s.log)
	assert.Error(t, err)
}

func TestStore_HintFileCreation(t *testing.T) {
	cfg := CreateDefaultConfig(TableKind_HashTbl, false)
	cfg.Compaction.MaxLogSize = 1024

	s := newTestStore(t, cfg)
	defer s.Close()

	readHintFile := func(id uint64) (max uint64, writable []uint64) {
		data, err := os.ReadFile(filepath.Join(s.tablePath, createHintName(id)))
		assert.NoError(t, err)

		for line := range strings.Lines(string(data)) {
			switch {
			case strings.HasPrefix(line, "largest: "):
				max, err = strconv.ParseUint(line[9:25], 16, 64)
				assert.NoError(t, err)
			case strings.HasPrefix(line, "writable: "):
				id, err = strconv.ParseUint(line[10:26], 16, 64)
				assert.NoError(t, err)
				writable = append(writable, id)
			}
		}
		return max, writable
	}

	// the first hint file should say any log file greater than 0 and no writable.
	max, writable := readHintFile(1)
	assert.Equal(t, max, 0)
	assert.Equal(t, writable, []uint64(nil))

	// compaction should create a new hint file, but it should be the same.
	s.AssertCompact(nil, time.Time{})
	max, writable = readHintFile(2)
	assert.Equal(t, max, 0)
	assert.Equal(t, writable, []uint64(nil))

	// creating data should say any log file greater than 1 and log file 1 is writable.
	s.AssertCreate()
	s.AssertCompact(nil, time.Time{})
	max, writable = readHintFile(3)
	assert.Equal(t, max, 1)
	assert.Equal(t, writable, []uint64{1})

	// after filling log file 1, it should say any log file greater than 2 and 2 is writable.
	for s.LogFile(s.AssertCreate()) == 1 {
	}
	s.AssertCompact(nil, time.Time{})
	max, writable = readHintFile(4)
	assert.Equal(t, max, 2)
	assert.Equal(t, writable, []uint64{2})
}

//
// benchmarks
//

func BenchmarkStore(b *testing.B) {
	forAllTables(b, benchmarkStore)
}
func benchmarkStore(b *testing.B, cfg Config) {
	ctx := b.Context()

	benchmarkSizes(b, "Create", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		s := newTestStore(b, cfg)
		defer s.Close()

		b.SetBytes(int64(size))
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()

		for i := 0; i < b.N; i++ {
			s.AssertCreate(WithData(buf))
			if s.Load() > db_CompactLoad {
				s.AssertCompact(nil, time.Time{})
			}
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	})

	benchmarkLRecs(b, "Compact", func(b *testing.B, lrec uint64) {
		s := newTestStore(b, cfg)
		defer s.Close()

		for i := uint64(0); i < 1<<lrec; i++ {
			s.AssertCreate(WithData(nil))
			if s.Load() > db_CompactLoad {
				s.AssertCompact(nil, time.Time{})
			}
		}
		s.AssertCompact(nil, time.Time{})

		b.ReportAllocs()
		b.ResetTimer()
		now := time.Now()

		for i := 0; i < b.N; i++ {
			var once sync.Once
			trashOne := func(ctx context.Context, key Key, created time.Time) (trash bool) {
				once.Do(func() { trash = true })
				return trash
			}
			assert.NoError(b, s.Compact(ctx, trashOne, time.Time{}))
		}

		b.ReportMetric(float64(b.N*int(1)<<lrec)/time.Since(now).Seconds(), "rec/sec")
	})

	b.Run("RewriteRecord", func(b *testing.B) {
		s := newTestStore(b, cfg)
		defer s.Close()

		key := s.AssertCreate(WithDataSize(200 * 1024))
		rec, ok, err := s.tbl.Lookup(ctx, key)
		assert.NoError(b, err)
		assert.True(b, ok)

		b.SetBytes(int64(rec.Length))
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			rec, err = s.rewriteRecord(ctx, rec, nil)
			assert.NoError(b, err)
		}
	})
}
