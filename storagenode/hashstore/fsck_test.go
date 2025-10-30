// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package hashstore

import (
	"slices"
	"testing"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestRecordTailFromLog(t *testing.T) {
	forAllTables(t, testRecordTailFromLog)
}
func testRecordTailFromLog(t *testing.T, cfg Config) {
	run := func(t *testing.T, count int, mutate func(t *testing.T, lf *logFile, i int)) {
		ctx := t.Context()

		s := newTestStore(t, cfg)
		defer s.Close()

		var lf *logFile
		var manual RecordTail
		var pushed RecordTail

		for i := range count {
			// create a new value and store it
			key := s.AssertCreate()

			// ensure everything is in one log
			id := s.LogFile(key)
			if lf == nil {
				lf, _ = s.lfs.Lookup(id)
			}
			assert.Equal(t, lf.id, id)

			// look up the record in the table
			rec, ok, err := s.tbl.Lookup(ctx, key)
			assert.True(t, ok)
			assert.NoError(t, err)

			// update our manually tracked tails
			if count-i-1 < len(RecordTail{}) {
				manual[count-i-1] = rec
			}
			pushed.Push(rec)

			// mutate the log file if requested
			if mutate != nil {
				mutate(t, lf, i)
			}
		}
		pushed.Sort()

		logTail, err := recordTailFromLog(ctx, lf, func(k Key, b []byte) bool { return true })
		assert.NoError(t, err)
		_, tails, err := OpenTable(ctx, s.tbl.Handle(), cfg)
		assert.NoError(t, err)

		assert.Equal(t, *tails[lf.id], *logTail)
		assert.Equal(t, manual, *logTail)
		assert.Equal(t, pushed, *logTail)
	}

	t.Run("Valid", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, true)()
		run(t, 2*len(RecordTail{}), nil)
	})

	t.Run("Small", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, true)()
		run(t, len(RecordTail{})/2, nil)
	})

	t.Run("WithGarbage", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, false)()
		run(t, 2*len(RecordTail{}), alwaysAddGarbage)
	})
}

func TestReadRecordsFromLogFile(t *testing.T) {
	run := func(
		t *testing.T,
		count int,
		mutate func(t *testing.T, lf *logFile, i int),
		check func(t *testing.T, lf *logFile, keys []Key),
	) {
		s := newTestStore(t, defaultConfig())
		defer s.Close()

		var lf *logFile
		var keys []Key

		for i := range count {
			// create a new value and store it
			key := s.AssertCreate()
			keys = append(keys, key)

			// ensure everything is in one log
			id := s.LogFile(key)
			if lf == nil {
				lf, _ = s.lfs.Lookup(id)
			}
			assert.Equal(t, lf.id, id)

			// mutate the log file if requested
			if mutate != nil {
				mutate(t, lf, i)
			}
		}

		slices.Reverse(keys)
		check(t, lf, keys)
	}

	t.Run("Basic", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, true)()
		run(t, 10, nil, func(t *testing.T, lf *logFile, keys []Key) {
			var got []Key
			assert.NoError(t, readRecordsFromLogFile(lf,
				func(k Key, b []byte) bool { return true },
				func(rec Record) bool { got = append(got, rec.Key); return true }))
			assert.Equal(t, got, keys)
		})
	})

	t.Run("WithGarbage", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, false)()
		run(t, 10, alwaysAddGarbage, func(t *testing.T, lf *logFile, keys []Key) {
			var got []Key
			assert.NoError(t, readRecordsFromLogFile(lf,
				func(k Key, b []byte) bool { return true },
				func(rec Record) bool { got = append(got, rec.Key); return true }))
			assert.Equal(t, got, keys)
		})
	})

	t.Run("Filtered", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, true)()
		run(t, 10, nil, func(t *testing.T, lf *logFile, keys []Key) {
			var parity bool
			var got []Key
			assert.NoError(t, readRecordsFromLogFile(lf,
				func(k Key, b []byte) bool { parity = !parity; return parity },
				func(rec Record) bool { got = append(got, rec.Key); return true }))
			assert.Equal(t, got, []Key{keys[0], keys[2], keys[4], keys[6], keys[8]})
		})
	})

	t.Run("SkipAll", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, true)()
		run(t, 10, nil, func(t *testing.T, lf *logFile, keys []Key) {
			assert.NoError(t, readRecordsFromLogFile(lf,
				func(k Key, b []byte) bool { return false },
				func(rec Record) bool { panic("should not be called") }))
		})
	})

	t.Run("GarbageDetected", func(t *testing.T) {
		defer temporarily(&test_fsck_errorOnInvalidRecord, true)()
		run(t, 1, alwaysAddGarbage, func(t *testing.T, lf *logFile, keys []Key) {
			assert.Error(t, readRecordsFromLogFile(lf,
				func(k Key, b []byte) bool { return true },
				func(rec Record) bool { panic("should not be called") }))
		})
	})
}

//
// shared helpers
//

func alwaysAddGarbage(t *testing.T, lf *logFile, i int) {
	buf := make([]byte, mwc.Intn(10))
	_, _ = mwc.Rand().Read(buf)
	n, err := lf.fh.Write(buf)
	assert.NoError(t, err)
	lf.size.Add(uint64(n))
}
