// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package hashstore

import (
	"context"
)

var (
	// if set to true, fsck returns an error if it encounters an invalid record in a log file.
	test_fsck_errorOnInvalidRecord = false
)

func recordTailFromLog(ctx context.Context, lf *logFile, valid func(Key, []byte) bool) (_ *RecordTail, err error) {
	defer mon.Task()(&ctx)(&err)

	rt, n := new(RecordTail), 0
	if err := readRecordsFromLogFile(lf, valid, func(rec Record) bool {
		rt.Push(rec)
		n++
		return n < len(rt)
	}); err != nil {
		return nil, err
	}
	rt.Sort()
	return rt, nil
}

func readRecordsFromLogFile(
	lf *logFile,
	valid func(Key, []byte) bool,
	cb func(Record) bool,
) (err error) {

	var contents []byte
	var buf [RecordSize]byte
	var rec Record

	size := lf.size.Load()

	for offset := int64(size) - RecordSize; offset >= 0; {
		if _, err := lf.fh.ReadAt(buf[:], offset); err != nil {
			return err
		}

		if !rec.ReadFrom(&buf) || rec.Log != lf.id || int64(rec.Offset)+int64(rec.Length) != offset {
			if test_fsck_errorOnInvalidRecord {
				return Error.New("invalid record in log file. rec:%v log:%q id:%d got:%d exp:%d",
					rec,
					lf.path,
					lf.id,
					int64(rec.Offset)+int64(rec.Length),
					offset,
				)
			}
			offset--
			continue
		}

		// we could skip reading the contents if valid is nil but in practice valid will always be
		// set.

		if len(contents) < int(rec.Length) {
			contents = make([]byte, rec.Length)
		}
		_, err = lf.fh.ReadAt(contents[:rec.Length], int64(rec.Offset))
		if err != nil {
			return err
		}
		if valid == nil || valid(rec.Key, contents[:rec.Length]) {
			if !cb(rec) {
				return nil
			}
		}

		offset = int64(rec.Offset) - RecordSize
	}

	return nil
}
