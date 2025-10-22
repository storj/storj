// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package hashstore

import (
	"testing"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestRecordTailFromLog(t *testing.T) {
	forAllTables(t, testRecordTailFromLog)
}
func testRecordTailFromLog(t *testing.T, cfg Config) {
	run := func(t *testing.T, count int, mutate func(lf *logFile)) {
		ctx := t.Context()

		s := newTestStore(t, cfg)
		defer s.Close()

		var lf *logFile
		var manual RecordTail
		var pushed RecordTail

		for i := range count {
			// create a new value and store it
			key := s.AssertCreate()
			createdId := s.LogFile(key)

			// ensure everything is in one log
			if lf == nil {
				lf, _ = s.lfs.Lookup(createdId)
			}
			assert.Equal(t, lf.id, createdId)

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
				mutate(lf)
			}
		}
		pushed.Sort()

		logTail, err := recordTailFromLog(ctx, lf, nil)
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
		run(t, 2*len(RecordTail{}), func(lf *logFile) {
			buf := make([]byte, mwc.Intn(10))
			_, _ = mwc.Rand().Read(buf)
			_, _ = lf.fh.Write(buf)
			lf.size.Add(uint64(len(buf)))
		})
	})
}
