// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package hashstore

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/storagenode/hashstore/platform"
)

var (
	// if set to true, fsck returns an error if it encounters an invalid record in a log file.
	test_fsck_errorOnInvalidRecord = false

	// if set to true, fsck uses the file-based log wrapper even in tests.
	test_fsck_skipMmapWrapper = false
)

// readRecordsFromLogFile reads all records from the given log file in reverse order (newest to
// oldest). For each record, it calls the valid function with the key and contents to determine if
// the record should be included. If valid returns true, the cb function is called with the record.
// If cb returns false, reading stops early.
func readRecordsFromLogFile(
	ctx context.Context,
	lf *logFile,
	valid func(Key, []byte) bool,
	cb func(Record) bool,
) (err error) {
	var contents []byte

	wr := wrapLogFile(lf)
	defer func() { err = errs.Combine(err, wr.Close()) }()

	for offset := int64(lf.size.Load()) - RecordSize; offset >= 0; {
		if err := ctx.Err(); err != nil {
			return err
		}

		rec, ok, err := wr.Record(offset)
		if err != nil {
			return err
		}

		if !ok || rec.Log != lf.id || int64(rec.Offset)+int64(rec.Length) != offset {
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
		_, err = wr.ReadAt(contents[:rec.Length], int64(rec.Offset))
		if err != nil {
			return err
		}
		if valid(rec.Key, contents[:rec.Length]) {
			if !cb(rec) {
				return nil
			}
		}

		offset = int64(rec.Offset) - RecordSize
	}

	return nil
}

// recordTailFromLog reads records from the given log file and constructs a RecordTail containing
// the newest records for each key. The valid function is used to filter which records are included.
func recordTailFromLog(ctx context.Context, lf *logFile, valid func(Key, []byte) bool) (_ *RecordTail, err error) {
	defer mon.Task()(&ctx)(&err)

	rt, n := new(RecordTail), 0
	if err := readRecordsFromLogFile(ctx, lf, valid, func(rec Record) bool {
		rt.Push(rec)
		n++
		return n < len(rt)
	}); err != nil {
		return nil, err
	}
	// if there are no entries, return nil because the hashtbl implementation will not have a tail
	// entry for logs that have no records.
	if n == 0 {
		return nil, nil
	}
	rt.Sort()
	return rt, nil
}

// logWrapper abstracts reading records from a log file, either via mmap or direct file reads.
type logWrapper interface {
	Close() error
	Record(off int64) (rec Record, ok bool, err error)
	ReadAt(p []byte, off int64) (n int, err error)
}

// wrapLogFile tries to create a mmap-based logWrapper, falling back to file-based if mmap fails.
func wrapLogFile(lf *logFile) logWrapper {
	if !test_fsck_skipMmapWrapper {
		m, err := platform.Mmap(lf.fh, int(lf.size.Load()))
		if err == nil {
			return &mmapLogWrapper{m: m}
		}
	}
	return &fileLogWrapper{lf: lf}
}

// mmapLogWrapper implements logWrapper using mmap.
type mmapLogWrapper struct {
	m []byte
}

func (m *mmapLogWrapper) Close() error { return platform.Munmap(m.m) }

func (m *mmapLogWrapper) Record(off int64) (rec Record, ok bool, err error) {
	if 0 <= off && off <= int64(len(m.m)) {
		if buf := m.m[off:]; len(buf) >= RecordSize {
			ok = rec.ReadFrom((*[RecordSize]byte)(buf))
		}
	}
	return rec, ok, nil
}

func (m *mmapLogWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	return copy(p, m.m[off:]), nil
}

// fileLogWrapper implements logWrapper using direct file reads.
type fileLogWrapper struct {
	lf *logFile
}

func (n *fileLogWrapper) Close() error { return nil }

func (n *fileLogWrapper) Record(off int64) (rec Record, ok bool, err error) {
	var buf [RecordSize]byte
	if _, err := n.lf.fh.ReadAt(buf[:], off); err != nil {
		return rec, false, err
	}
	ok = rec.ReadFrom(&buf)
	return rec, ok, nil
}

func (n *fileLogWrapper) ReadAt(p []byte, off int64) (nread int, err error) {
	return n.lf.fh.ReadAt(p, off)
}
