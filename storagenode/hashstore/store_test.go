// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestStore_BasicOperation(t *testing.T) {
	ctx := context.Background()

	s := newTestStore(t, 8)
	defer s.Close()

	var keys []Key

	// write a bunch of keys and compact a couple of times.
	for i := 0; i < 4; i++ {
		for j := 0; j < 1024; j++ {
			key := s.AssertCreate(time.Time{})
			keys = append(keys, key)
			s.AssertRead(key)
		}
		s.AssertCompact(nil, time.Time{})
	}

	// ensure we can still read all of the keys even after compaction.
	for _, key := range keys {
		s.AssertRead(key)
	}

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

func TestStore_FileLocking(t *testing.T) {
	if !flockSupported {
		t.Skip("flock not supported on this platform")
	}

	dir := t.TempDir()

	s, err := newStore(dir, 1, nil)
	assert.NoError(t, err)
	defer s.Close()

	_, err = newStore(dir, 1, nil)
	assert.Error(t, err)
}

func TestStore_CreateSameKeyErrors(t *testing.T) {
	s := newTestStore(t, 1)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate(time.Time{})

	// attempting to make the same entry fails on the Close call.
	wr, err := s.Create(context.Background(), key, time.Time{})
	assert.NoError(t, err)
	assert.Error(t, wr.Close())
}

func TestStore_ReadFromCompactedFile(t *testing.T) {
	s := newTestStore(t, 1)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate(time.Time{})

	// grab a reader for the key and hold on to it through compaction.
	r, err := s.Read(context.Background(), key)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	defer r.Release()

	// compact the store so that it is flagged as trash.
	s.AssertCompact(alwaysTrash, time.Time{})

	// move to the future so that compaction deletes the record.
	s.today += compaction_ExpiresDays + 1
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
	s := newTestStore(t, 1)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate(time.Time{})

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
	s := newTestStore(t, 1)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate(time.Time{})

	// flag the key as trash.
	s.AssertCompact(alwaysTrash, time.Time{})

	// assume a restore call came in today.
	restore := dateToTime(s.today)

	// compact again far enough ahead to ensure it would be deleted if not for restore.
	s.today += compaction_ExpiresDays + 1
	s.AssertCompact(nil, restore)

	// grab a reader for the key. it should still exist.
	s.AssertRead(key)
}

func TestStore_TTL(t *testing.T) {
	s := newTestStore(t, 1)
	defer s.Close()

	// add an entry to the store that is already expired.
	key := s.AssertCreate(time.Now())

	// compact the store so that the expired key is deleted.
	s.today += 3 // 3 just in case the test is running near midnight.
	s.AssertCompact(nil, time.Time{})

	// grab a reader for the key. it should be deleted.
	s.AssertNotExist(key)
}

func TestStore_CompactionWithTTLTakesShorterTime(t *testing.T) {
	t.Run("CompactionShorter", func(t *testing.T) {
		s := newTestStore(t, 1)
		defer s.Close()

		// add an entry to the store that will expire way in the future.
		key := s.AssertCreate(time.Now().Add(24 * time.Hour * 10 * compaction_ExpiresDays))

		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// bump time to the minimum necessary to expire the key.
		s.today += compaction_ExpiresDays + 1
		s.AssertCompact(nil, time.Time{})

		// the key should not exist.
		s.AssertNotExist(key)
	})

	t.Run("TTLShorter", func(t *testing.T) {
		s := newTestStore(t, 1)
		defer s.Close()

		// add an entry to the store that is already expired.
		key := s.AssertCreate(time.Now())

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
	s := newTestStore(t, 1)
	defer s.Close()

	now := time.Now()

	// add some entries to the store that are already expired so that the log file will have enough
	// dead data to be compacted.
	var expired []Key
	for i := 0; i < 10; i++ {
		expired = append(expired, s.AssertCreate(now))
	}

	// add some entries to the store that are not expired. keep track of their records in the
	// hashtbl so that we can ensure they are in a new log file after compaction.
	var live []Key
	var recs []record
	for i := 0; i < 10; i++ {
		key := s.AssertCreate(time.Time{})
		live = append(live, key)

		rec, ok, err := s.tbl.Lookup(key)
		assert.NoError(t, err)
		assert.True(t, ok)
		recs = append(recs, rec)
	}

	// compact the store so that the expired key is deleted.
	s.today += 3 // 3 just in case the test is running near midnight.
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

		assert.False(t, recordsEqualish(got, exp)) // ensure they are different.
		got.log, exp.log = 0, 0
		got.offset, exp.offset = 0, 0
		assert.True(t, recordsEqualish(got, exp)) // but only in their log location.
	}
}

func TestStore_WriteCancel(t *testing.T) {
	s := newTestStore(t, 1)
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
			assert.NoError(t, wr.Close())
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
	s.lfs.Range(func(_ uint32, lf *logFile) bool {
		assert.False(t, looped.set())
		assert.Equal(t, lf.size, 0)
		size, err := fileSize(lf.fh)
		assert.NoError(t, err)
		assert.Equal(t, size, 1024)
		return true
	})
}

func TestStore_ReadRevivesTrash(t *testing.T) {
	s := newTestStore(t, 1)
	defer s.Close()

	// add an entry to the store that does not expire.
	key := s.AssertCreate(time.Time{})

	for i := 0; i < 5*compaction_ExpiresDays; i++ {
		// flag the key as trash.
		s.AssertCompact(alwaysTrash, time.Time{})

		// grab a reader for the key. it should still exist.
		s.AssertRead(key)

		// move on to the next day.
		s.today++
	}
}

func TestStore_MergeRecordsWhenCompactingWithLostPage(t *testing.T) {
	ctx := context.Background()

	s := newTestStore(t, 1)
	defer s.Close()

	// create two keys that collide at the end of the first page.
	k0 := Key{0: rPerP - 1}
	k1 := Key{0: rPerP - 1, 31: 1}

	// write k0 and k1 to the store.
	s.AssertCreateKey(k0, time.Time{})
	s.AssertCreateKey(k1, time.Time{})

	// compact the store flagging k1 as trash.
	assert.NoError(t, s.Compact(ctx, func(ctx context.Context, key Key, created time.Time) (bool, error) {
		return key == k1, nil
	}, time.Time{}))

	// clear out the first page so that any updates to k1 don't overwrite the existing entry for k1.
	_, err := s.tbl.fh.WriteAt(make([]byte, pSize), 0)
	assert.NoError(t, err)
	s.tbl.invalidatePageCache()

	// reading k1 will cause it to revive, adding the duplicate entry for k1.
	s.AssertRead(k1)

	// ensure the only entries in the table are duplicate k1 entries.
	count := 0
	s.tbl.Range(func(rec record, err error) bool {
		assert.NoError(t, err)
		assert.Equal(t, rec.key, k1)
		count++
		return true
	})
	assert.Equal(t, count, 2)

	// when we compact, it should take the later expiration for k1 so it will never delete it.
	s.AssertCompact(nil, time.Time{})

	// bump the day so that if it were to delete k1, it would have.
	s.today += compaction_ExpiresDays + 1
	s.AssertCompact(nil, time.Time{})

	// k1 should still be reachable.
	s.AssertRead(k1)
}

func TestStore_ReviveDuringCompaction(t *testing.T) {
	run := func(t *testing.T, future uint32) {
		ctx := context.Background()

		s := newTestStore(t, 1)
		defer s.Close()

		// insert the key we'll be reviving.
		key := s.AssertCreate(time.Time{})

		// compact the store so that the key is trashed.
		s.AssertCompact(alwaysTrash, time.Time{})

		// insert a 2nd key that we will have to call the trash callback on so we can control the
		// progress of the compaction.
		s.AssertCreate(time.Time{})

		// potentially go into the future so that the key is maybe deleted.
		s.today += future

		// begin a compaction in the background that we can control when it proceeds with the trash
		// callback.
		activity := make(chan bool)
		errCh := make(chan error)

		go func() {
			errCh <- s.Compact(ctx,
				func(ctx context.Context, key Key, created time.Time) (bool, error) {
					for !<-activity { // wait until we are sent true to continue.
					}
					return false, nil
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
			activity <- true
		}()

		// try to read the key which will attempt to revive it while compaction is running.
		s.AssertRead(key)

		// compaction should finish without error.
		assert.NoError(t, <-errCh)

		// revive could have done nothing. after compaction is finished we can read again to ensure
		// it was actually revived.
		s.AssertRead(key)
	}

	t.Run("Dead", func(t *testing.T) { run(t, compaction_ExpiresDays+1) })
	t.Run("Alive", func(t *testing.T) { run(t, 0) })
}

func TestStore_CloseCancelsCompaction(t *testing.T) {
	s := newTestStore(t, 1)
	defer s.Close()

	// insert some keys for compaction to attempt to process.
	for i := 0; i < 10; i++ {
		s.AssertCreate(time.Time{})
	}

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(context.Background(),
			func(ctx context.Context, key Key, created time.Time) (bool, error) {
				for !<-activity { // wait until we are sent true to continue.
				}
				<-ctx.Done() // wait for the context to be canceled.
				return false, nil
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
	s := newTestStore(t, 1)
	defer s.Close()

	// insert a key for compaction to attempt to process.
	s.AssertCreate(time.Time{})

	// begin a compaction in the background that we can control when it proceeds with the trash
	// callback.
	activity := make(chan bool)
	errCh := make(chan error)

	go func() {
		errCh <- s.Compact(context.Background(),
			func(ctx context.Context, key Key, created time.Time) (bool, error) {
				for !<-activity { // wait until we are sent true to continue.
				}
				return false, nil
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

//
// benchmarks
//

func BenchmarkStore(b *testing.B) {
	ctx := context.Background()

	benchmarkSizes(b, "Create", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		s := newTestStore(b, 8)
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

		s := newTestStore(b, runtime.GOMAXPROCS(0))
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
}
