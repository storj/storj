// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestStore_BasicOperation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
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
	assert.That(t, stats.Table.LenSet <= stats.LenLogs) // <= because of optimistic alignment
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
	s := newTestStore(t)
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
	if !flockSupported {
		t.Skip("flock not supported on this platform")
	}

	s := newTestStore(t)
	defer s.Close()

	// flock should stop a second store from being created with the same hashdir.
	_, err := NewStore(s.dir, nil)
	assert.Error(t, err)

	// it should still be locked even after compact makes a new hashtbl file.
	s.AssertCompact(nil, time.Time{})
	_, err = NewStore(s.dir, nil)
	assert.Error(t, err)
}

func TestStore_CreateSameKeyErrors(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// attempting to make the same entry fails on the Close call.
	wr, err := s.Create(context.Background(), key, time.Time{})
	assert.NoError(t, err)
	assert.Error(t, wr.Close())
}

func TestStore_ReadFromCompactedFile(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// add some already expired entries to the store so the log file is compacted.
	for i := 0; i < 100; i++ {
		s.AssertCreate(WithTTL(time.Now().Add(-100 * 24 * time.Hour)))
	}

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// grab the record for the key so we can compare it to the record after compaction.
	before, ok, err := s.tbl.Lookup(key)
	assert.NoError(t, err)
	assert.True(t, ok)

	// grab a reader for the key and hold on to it through compaction.
	r, err := s.Read(context.Background(), key)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	defer r.Release()

	// compact the store so that it is flagged as trash.
	s.AssertCompact(alwaysTrash, time.Time{})

	// ensure that the log file for the record changed and the original log file was compacted.
	after, ok, err := s.tbl.Lookup(key)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.That(t, before.Log < after.Log)

	// move to the future so that compaction deletes the record.
	s.today += compaction_ExpiresDays + 1 // 1 more just in case the test is running near midnight.
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
	s := newTestStore(t)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// compact a bunch of times, every day incrementing by one. we need to do two extra days because
	// the first compaction flags it to be deleted after ExpiresDays, we then need to wait that many
	// days, and then the next compaction will actually delete it.
	for i := 0; i < 1+compaction_ExpiresDays+1; i++ {
		s.AssertCompact(alwaysTrash, time.Time{})
		s.today++
	}

	// grab a reader for the key. it should be deleted.
	s.AssertNotExist(key)
}

func TestStore_CompactionRespectsRestoreTime(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	// flag the key as trash.
	s.AssertCompact(alwaysTrash, time.Time{})

	// assume a restore call came in today.
	restore := DateToTime(s.today)

	// compact again far enough ahead to ensure it would be deleted if not for restore.
	s.today += compaction_ExpiresDays + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, restore)

	// grab a reader for the key. it should still exist.
	s.AssertRead(key)
}

func TestStore_TTL(t *testing.T) {
	s := newTestStore(t)
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
	t.Run("CompactionShorter", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()

		// add an entry to the store that will expire way in the future.
		key := s.AssertCreate(WithTTL(time.Now().Add(24 * time.Hour * 10 * compaction_ExpiresDays)))

		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// bump time to the minimum necessary to expire the key.
		s.today += compaction_ExpiresDays + 1 // 1 more just in case the test is running near midnight.
		s.AssertCompact(nil, time.Time{})

		// the key should not exist.
		s.AssertNotExist(key)
	})

	t.Run("TTLShorter", func(t *testing.T) {
		s := newTestStore(t)
		defer s.Close()

		// add an entry to the store that is already expired.
		key := s.AssertCreate(WithTTL(time.Now()))

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
	s := newTestStore(t)
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

		rec, ok, err := s.tbl.Lookup(key)
		assert.NoError(t, err)
		assert.True(t, ok)
		recs = append(recs, rec)
	}

	// compact the store so that the expired keys are deleted.
	s.today += compaction_ExpiresDays + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})

	// all the expired keys should be deleted.
	for _, key := range expired {
		s.AssertNotExist(key)
	}

	// all the live keys should still exist, but be in a new log file.
	for n, key := range live {
		s.AssertRead(key)
		exp := recs[n]

		got, ok, err := s.tbl.Lookup(key)
		assert.NoError(t, err)
		assert.True(t, ok)

		assert.False(t, RecordsEqualish(got, exp)) // ensure they are different.
		got.Log, exp.Log = 0, 0
		got.Offset, exp.Offset = 0, 0
		assert.True(t, RecordsEqualish(got, exp)) // but only in their log location.
	}
}

func TestStore_ClumpObjectsByTTL(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	check := func(key Key, log int) {
		rec, ok, err := s.tbl.Lookup(key)
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
	s := newTestStore(t)
	defer s.Close()

	// grab a writer for the key and cancel it a bunch.
	var keys []Key
	for i := 0; i < 1000; i++ {
		key := newKey()
		keys = append(keys, key)

		wr, err := s.Create(context.Background(), key, time.Time{})
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
	s.lfs.Range(func(_ uint64, lf *logFile) bool {
		assert.False(t, looped.set())
		assert.Equal(t, lf.size.Load(), 0)
		size, err := fileSize(lf.fh)
		assert.NoError(t, err)
		assert.Equal(t, size, 1024)
		return true
	})
}

func TestStore_ReadRevivesTrash(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate()

	for i := 0; i < 5*compaction_ExpiresDays; i++ {
		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// grab a reader for the key. it should still exist.
		s.AssertRead(key)

		// move on to the next day.
		s.today++
	}

	// ensure the Trash flag is set.
	s.AssertCompact(alwaysTrash, time.Time{})
	r, err := s.Read(context.Background(), key)
	defer r.Release()
	assert.NoError(t, err)
	assert.True(t, r.Trash())
}

func TestStore_MergeRecordsWhenCompactingWithLostPage(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	// helper function to create a key that goes into the given page and record index. n is used to
	// create distinct keys with the same page and record index.
	createKey := func(pi, ri uint64, n uint8) (k Key) {
		rng := mwc.Rand()
		for {
			binary.BigEndian.PutUint64(k[0:8], rng.Uint64())
			k[31] = n
			gpi, gri := s.tbl.pageAndRecordIndexForSlot(s.tbl.slotForKey(&k))
			if pi == gpi && ri == gri {
				return k
			}
		}
	}

	// create two keys that collide at the end of the first page.
	k0 := createKey(0, recordsPerPage-1, 0)
	k1 := createKey(0, recordsPerPage-1, 1)

	// write k0 and k1 to the store.
	s.AssertCreateKey(k0, time.Time{})
	s.AssertCreateKey(k1, time.Time{})

	// create a large key in the third page so that the log file is kept alive.
	kl := createKey(2, 0, 0)

	w, err := s.Create(ctx, kl, time.Time{})
	assert.NoError(t, err)
	_, err = w.Write(make([]byte, 10*1024))
	assert.NoError(t, err)
	assert.NoError(t, w.Close())

	// compact the store flagging k1 as trash.
	assert.NoError(t, s.Compact(ctx, func(_ context.Context, key Key, _ time.Time) bool {
		return key == k1
	}, time.Time{}))

	// clear out the first page so that any updates to k1 don't overwrite the existing entry for k1.
	_, err = s.tbl.fh.WriteAt(make([]byte, pageSize), pageSize) // offset=pSize to skip the header page
	assert.NoError(t, err)

	// reading k1 will cause it to revive, adding the duplicate entry for k1.
	s.AssertRead(k1)

	// ensure the only entries in the table are duplicate k1 entries and kl.
	keys := []Key{k1, k1, kl}
	s.tbl.Range(func(rec Record, err error) bool {
		assert.NoError(t, err)
		assert.Equal(t, rec.Key, keys[0])
		keys = keys[1:]
		return true
	})
	assert.Equal(t, len(keys), 0)

	// when we compact, it should take the later expiration for k1 so it will never delete it.
	s.AssertCompact(nil, time.Time{})

	// bump the day so that if it were to delete k1, it would have.
	s.today += compaction_ExpiresDays + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})

	// k1 should still be reachable.
	s.AssertRead(k1)
}

func TestStore_ReviveDuringCompaction(t *testing.T) {
	run := func(t *testing.T, future uint32) {
		ctx := context.Background()
		s := newTestStore(t)
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
				"TestStore_ReviveDuringCompaction",
				"Create",
			)
			// the following AssertRead call is blocked on Create, allow compaction to finish.
			close(activity)
		}()

		// try to read the key which will attempt to revive it while compaction is running.
		s.AssertRead(key)

		// compaction should finish without error.
		assert.NoError(t, <-errCh)

		// revive could have done nothing. after compaction is finished we can read again to ensure
		// it was actually revived.
		s.AssertRead(key, AssertTrash(false))
	}

	t.Run("Dead", func(t *testing.T) { run(t, compaction_ExpiresDays+1) })
	t.Run("Alive", func(t *testing.T) { run(t, 0) })
}

func TestStore_MultipleReviveDuringCompaction(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
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

	// start a goroutine that waits for 2 stacks to be blocked in reviveRecord where one of them
	// is blocked trying to Create to recreate the trashed record and allow compaction to continue.
	go func() {
		waitForGoroutines(2,
			"(*Store).reviveRecord",
			"(*mutex).Lock",
		)
		waitForGoroutine(
			"(*Store).reviveRecord",
			"(*Store).Create",
			"(*rwMutex).RLock",
			"(*mutex).Lock",
		)
		close(activity)
	}()

	// start functions reading the two keys that will do the revive.
	read := func(k Key) {
		r, err := s.Read(ctx, k)
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
	s := newTestStore(t)
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
		errCh <- s.Compact(context.Background(),
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

	// launch a goroutine that confirms that this test has a Create call blocked in Create then
	// closes the store.
	go func() {
		waitForGoroutine(
			"TestStore_CloseCancelsCompaction",
			"Create",
		)
		s.Close()
	}()

	// try to create a key and ensure it fails.
	_, err := s.Create(context.Background(), newKey(), time.Time{})
	assert.Error(t, err)

	// compaction should have errored.
	assert.Error(t, <-errCh)
}

func TestStore_ContextCancelsCreate(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// insert a key for compaction to attempt to process.
	s.AssertCreate()

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(context.Background(),
			func(ctx context.Context, key Key, created time.Time) bool {
				for !<-activity { // wait until we are sent true to continue.
				}
				return false
			}, time.Time{})
	}()

	// wait until the compaction is asking to trash our key and allow it to proceed to block on the
	// context being canceled.
	activity <- false

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// launch a goroutine that confirms that this test has a Create call blocked in Create then
	// cancels the context.
	go func() {
		waitForGoroutine(
			"TestStore_ContextCancelsCreate",
			"Create",
		)
		cancel()
	}()

	// try to create a key and ensure it fails.
	_, err := s.Create(ctx, newKey(), time.Time{})
	assert.Error(t, err)

	// allow compaction to finish.
	activity <- true

	// compaction should not have errored.
	assert.NoError(t, <-errCh)
}

func TestStore_LogContainsDataToReconstruct(t *testing.T) {
	const parallelism = 4

	ctx := context.Background()
	s := newTestStore(t)
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

	// add a canceled record to the end of some log file to ensure that we can still reconstruct
	// the table.
	w, err := s.Create(ctx, newKey(), time.Time{})
	assert.NoError(t, err)
	buf := make([]byte, 128)
	_, _ = mwc.Rand().Read(buf)
	_, err = w.Write(buf)
	assert.NoError(t, err)
	w.Cancel()

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
	s.lfs.Range(func(_ uint64, lf *logFile) bool {
		lfRecs = append(lfRecs, collectRecords(lf)...)
		return true
	})

	// collect all the records in the hash table.
	var tblRecs []Record
	s.tbl.Range(func(rec Record, err error) bool {
		assert.NoError(t, err)
		tblRecs = append(tblRecs, rec)
		return true
	})

	// both sets of records should be equal.
	sort.Slice(lfRecs, func(i, j int) bool {
		return string(lfRecs[i].Key[:]) < string(lfRecs[j].Key[:])
	})
	sort.Slice(tblRecs, func(i, j int) bool {
		return string(tblRecs[i].Key[:]) < string(tblRecs[j].Key[:])
	})
	assert.DeepEqual(t, lfRecs, tblRecs)
}

func TestStore_OptimisticAlignment(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	w, err := s.Create(context.Background(), newKey(), time.Time{})
	assert.NoError(t, err)

	// write enough so that after the footer record is appended, we only need to add 10 bytes to the
	// file to align it to 4k
	_, err = w.Write(make([]byte, 4096-RecordSize-10))
	assert.NoError(t, err)
	assert.NoError(t, w.Close())

	stats := s.Stats()
	assert.Equal(t, stats.Table.LenSet, 4096-10) // alive data is 4096-rSize-10 + rSize.
	assert.Equal(t, stats.LenLogs, 4096)         // total data should be aligned up to 4k.
}

func TestStore_RaceConcurrentWriteAndStats(t *testing.T) {
	s := newTestStore(t)
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
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	getSize := func() (size uint64) {
		s.lfs.Range(func(_ uint64, lf *logFile) bool {
			size = lf.size.Load()
			return false
		})
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
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	// we are testing when compaction is trying to rewrite a log file that
	// contains more used data than the next hashtbl. we'll do this by having
	// a single large (multi-MB) entry and many small but dead entries and
	// triggering compaction.

	writeEntry := func(size int64) Key {
		key := newKey()
		w, err := s.Create(ctx, key, time.Time{})
		assert.NoError(t, err)
		_, err = w.Write(make([]byte, size))
		assert.NoError(t, err)
		assert.NoError(t, w.Close())
		return key
	}

	large := writeEntry(10 << 20)
	for i := 0; i < 1<<10; i++ {
		writeEntry(10 << 10)
	}
	s.AssertCompact(func(ctx context.Context, key Key, created time.Time) bool {
		return key != large
	}, time.Time{})

	// compact the store so that the expired key is deleted.
	s.today += compaction_ExpiresDays + 1 // 1 more just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})
}

func TestStore_CompactionExitsEarlyWhenNoModifications(t *testing.T) {
	s := newTestStore(t)
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
	s := newTestStore(t)
	defer s.Close()

	getLog := func(key Key) uint64 {
		rec, ok, err := s.tbl.Lookup(key)
		assert.NoError(t, err)
		assert.True(t, ok)
		return rec.Log
	}

	// add a key that goes into a non-ttl log file.
	permKey := s.AssertCreate()

	// create the log file that the ttl key would go into outside of the knowledge of the store so
	// that it fails when trying to create it itself.
	now := time.Now()
	ttl := TimeToDateUp(now)
	id := getLog(permKey) + 1
	dir := filepath.Join(s.dir, fmt.Sprintf("%02x", byte(id)))
	assert.NoError(t, os.MkdirAll(dir, 0755))
	name := filepath.Join(dir, fmt.Sprintf("log-%016x-%08x", id, ttl))
	fh, err := os.OpenFile(name, os.O_CREATE, 0)
	assert.NoError(t, err)
	assert.NoError(t, fh.Close())

	// add a key that goes into a ttl log file. it should succeed anyway.
	ttlKey := s.AssertCreate(now)

	// they should be in the same log file.
	assert.Equal(t, getLog(permKey), getLog(ttlKey))
}

func TestStore_HashtblFull(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	defer s.Close()

	for {
		w, err := s.Create(ctx, newKey(), time.Time{})
		assert.NoError(t, err)
		if err := w.Close(); err != nil {
			break
		}
	}

	assert.Equal(t, s.Stats().TableFull, 1)
}

//
// benchmarks
//

func BenchmarkStore(b *testing.B) {
	ctx := context.Background()

	benchmarkSizes(b, "Create", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		s := newTestStore(b)
		defer s.Close()

		b.SetBytes(int64(size))
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()

		for i := 0; i < b.N; i++ {
			wr, err := s.Create(ctx, newKey(), time.Time{})
			assert.NoError(b, err)

			_, err = wr.Write(buf)
			assert.NoError(b, err)
			assert.NoError(b, wr.Close())

			if l := s.Load(); l > 0.5 {
				assert.NoError(b, s.Compact(ctx, nil, time.Time{}))
			}
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	})

	benchmarkSizes(b, "CreateParallel", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		s := newTestStore(b)
		defer s.Close()

		b.SetBytes(int64(size))
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()
		cmu := new(sync.Mutex)

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				wr, err := s.Create(ctx, newKey(), time.Time{})
				assert.NoError(b, err)

				_, err = wr.Write(buf)
				assert.NoError(b, err)
				assert.NoError(b, wr.Close())

				if l := s.Load(); l > 0.5 {
					cmu.Lock()
					if s.Load() > 0.5 {
						assert.NoError(b, s.Compact(ctx, nil, time.Time{}))
					}
					cmu.Unlock()
				}
			}
		})

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	})

	benchmarkLRecs(b, "Compact", func(b *testing.B, lrec uint64) {
		s := newTestStore(b)
		defer s.Close()

		for i := uint64(0); i < 1<<lrec; i++ {
			s.AssertCreate()
			if s.Load() > 0.5 {
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

}
