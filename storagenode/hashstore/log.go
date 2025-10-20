// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"

	"github.com/zeebo/errs"
)

var (
	// if set to true, the store does extra checks to ensure log file sizes match their seek offset.
	test_Log_CheckSizeAndOffset = false
)

// logFile represents a ref-counted handle to a log file that stores piece data.
type logFile struct {
	// immutable fields
	path string
	id   uint64
	ttl  uint32
	fh   *os.File

	// mutable fields
	size   atomic.Uint64
	closed flag
	err    error // saved error from fh.Close
}

func newLogFile(path string, id uint64, ttl uint32, fh *os.File, size uint64) *logFile {
	lf := &logFile{
		path: path,
		id:   id,
		ttl:  ttl,
		fh:   fh,
	}
	lf.size.Store(size)
	return lf
}

func (lf *logFile) Close() error {
	if !lf.closed.set() {
		lf.err = errs.Combine(
			lf.fh.Sync(),
			lf.fh.Close(),
		)
	}
	return lf.err
}

func (lf *logFile) Closed() bool {
	return lf.closed.get()
}

//
// heap of log files by size
//

type logHeap []*logFile

func (h logHeap) Len() int           { return len(h) }
func (h logHeap) Less(i, j int) bool { return h[i].size.Load() > h[j].size.Load() }
func (h logHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *logHeap) Push(x any)        { *h = append(*h, x.(*logFile)) }
func (h *logHeap) Pop() any {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[:n-1]
	return x
}

//
// a collection of log files
//

type logCollection struct {
	mu  sync.Mutex
	lfs map[uint32]*logHeap
}

func newLogCollection() *logCollection {
	return &logCollection{
		lfs: make(map[uint32]*logHeap),
	}
}

func (l *logCollection) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	clear(l.lfs)
}

func (l *logCollection) Include(lf *logFile) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// if the log has been closed, we can't write to it so don't include it.
	if lf.Closed() {
		return
	}

	lfh := l.lfs[lf.ttl]
	if lfh == nil {
		lfh = new(logHeap)
		l.lfs[lf.ttl] = lfh
	}

	heap.Push(lfh, lf)
}

func (l *logCollection) Acquire(ttl uint32) *logFile {
	l.mu.Lock()
	defer l.mu.Unlock()

	lfh := l.lfs[ttl]
	if lfh == nil || lfh.Len() == 0 {
		return nil
	}
	return heap.Pop(lfh).(*logFile)
}

//
// Reader
//

// Reader is a type that reads a section from a log file.
type Reader struct {
	s    *Store
	r    *io.SectionReader
	path string
	fh   *os.File
	rec  Record
}

func newLogReader(s *Store, path string, fh *os.File, rec Record) *Reader {
	return &Reader{
		s:    s,
		r:    io.NewSectionReader(fh, int64(rec.Offset), int64(rec.Length)),
		path: path,
		fh:   fh,
		rec:  rec,
	}
}

// Revive attempts to revive a trashed piece.
func (l *Reader) Revive(ctx context.Context) error {
	if !l.Trash() {
		return nil
	}
	return l.s.reviveRecord(ctx, l.fh, l.rec)
}

// Key returns the key of thereader.
func (l *Reader) Key() Key { return l.rec.Key }

// Size returns the size of the reader.
func (l *Reader) Size() int64 { return int64(l.rec.Length) }

// Trash returns true if the reader was for a trashed piece.
func (l *Reader) Trash() bool { return l.rec.Expires.Trash() }

// Seek implements io.Seeker.
func (l *Reader) Seek(offset int64, whence int) (int64, error) { return l.r.Seek(offset, whence) }

// ReadAt implements io.ReaderAt.
func (l *Reader) ReadAt(p []byte, off int64) (int, error) { return l.r.ReadAt(p, off) }

// Read implements io.Reader.
func (l *Reader) Read(p []byte) (int, error) { return l.r.Read(p) }

// Release returns the resources associated with the reader. It must be called when done.
func (l *Reader) Release() { _ = l.Close() }

// Close is like Release but implements io.Closer. The returned error is always nil.
func (l *Reader) Close() error { l.s.lru.Put(l.path, l.fh); return nil }

//
// Writer
//

// Writer is a type that allows one to write a piece to a log file.
type Writer struct {
	ctx   context.Context
	store *Store

	mu       sync.Mutex // protects the following fields
	canceled flag
	closed   flag
	rec      Record
	buf      []byte
}

func newWriter(ctx context.Context, s *Store, key Key, expires Expiration) *Writer {
	return &Writer{
		ctx:   ctx,
		store: s,

		rec: Record{
			Key:     key,
			Created: s.today(),
			Expires: expires,
		},
	}
}

// Size returns the number of bytes written to the Writer.
func (h *Writer) Size() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return int64(h.rec.Length)
}

// Close commits the writes that have happened. Close or Cancel must be called at least once.
func (h *Writer) Close() (err error) {
	ctx := h.ctx
	defer mon.Task()(&ctx)(&err)

	// if we are not the first to close or we are canceled, do nothing.
	h.mu.Lock()
	closed := h.closed.set()
	canceled := h.canceled.get()
	h.mu.Unlock()

	if canceled {
		return Error.New("already canceled")
	} else if closed {
		return nil
	}

	// attempt to acquire the flush semaphore from the store.
	if err := h.store.flushMu.RLock(h.ctx, &h.store.closed); err != nil {
		return Error.Wrap(err)
	}
	defer h.store.flushMu.RUnlock()

	// acquire a log file to write the data into.
	lf, err := h.store.acquireLogFile(h.rec.Expires.Time())
	if err != nil {
		return Error.Wrap(err)
	}
	defer h.store.lfc.Include(lf)

	// set ourselves to the end offset so that we are sure we have the correct spot for the record.
	offset, err := lf.fh.Seek(0, io.SeekEnd)
	if err != nil {
		return Error.Wrap(err)
	}

	// if we're testing the size and offset, ensure they match.
	if test_Log_CheckSizeAndOffset {
		if size := lf.size.Load(); size != uint64(offset) {
			panic(fmt.Sprintf("log file size=%d and offset=%d mismatch for %q",
				size, offset, lf.path))
		}
	}

	// update the record fields to point at the correct place in the log file.
	h.rec.Log = lf.id
	h.rec.Offset = uint64(offset)

	// append the record to the end of the data so that the log can be used for reconstruction.
	var buf [RecordSize]byte
	h.rec.WriteTo(&buf)
	h.buf = append(h.buf, buf[:]...)

	// write the buffer to the log file.
	if _, err := lf.fh.Write(h.buf); err != nil {
		// if we couldn't write the piece data or potentially just the record for reconstruction, we
		// should abort the write operation and attempt to reclaim space by truncating to the saved
		// offset.
		_ = lf.fh.Truncate(offset)
		return Error.Wrap(err)
	}

	// add the record to the store.
	if err := h.store.addRecord(ctx, h.rec); err != nil {
		// if we can't add the record, we should abort the write operation and attempt to reclaim
		// space by truncating to the saved offset.
		_ = lf.fh.Truncate(offset)
		return Error.Wrap(err)
	}

	// increase our in-memory estimate of the size of the log file for sorting. we use store to
	// ensure that it maintains correctness if there were some errors in the past.
	lf.size.Store(uint64(offset) + uint64(len(h.buf)))

	// drop the memory early in case someone holds on to the closed writer.
	h.buf = nil

	// sync the log file and tbl if we're syncing every write. we don't seek back if the sync
	// fails because it's unclear what state anything is in anyway.
	if h.store.cfg.Store.SyncWrites {
		err := errs.Combine(
			Error.Wrap(lf.fh.Sync()),
			h.store.tbl.Sync(ctx),
		)
		if err != nil {
			return err
		}
	}

	// if the size is over the max size, close the file handle.
	if lf.size.Load() >= h.store.cfg.Compaction.MaxLogSize {
		if err := lf.Close(); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// Cancel discards the writes that have happened. Close or Cancel must be called at least once.
func (h *Writer) Cancel() {
	ctx := h.ctx
	defer mon.Task()(&ctx)(nil)

	// if we are not the first to cancel or we are closed, do nothing.
	h.mu.Lock()
	closed := h.closed.get()
	canceled := h.canceled.set()
	h.mu.Unlock()

	if closed || canceled {
		return
	}

	// drop the memory early in case someone holds on to the canceled writer.
	h.buf = nil
}

// Write implements io.Writer.
func (h *Writer) Write(p []byte) (_ int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.canceled || h.closed {
		return 0, Error.New("invalid handle")
	} else if uint64(h.rec.Length)+uint64(len(p)) > math.MaxUint32 {
		return 0, Error.New("piece too large")
	}

	// optimize for the common single piece data write + 512 byte footer + record
	if h.buf == nil {
		h.buf = make([]byte, 0, len(p)+512+RecordSize)
	}

	h.buf = append(h.buf, p...)
	h.rec.Length += uint32(len(p))

	return len(p), err
}
