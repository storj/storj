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

// logFile represents a ref-counted handle to a log file that stores piece data.
type logFile struct {
	// immutable fields
	fh  *os.File
	id  uint64
	ttl uint32

	// atomic fields
	size atomic.Uint64

	// mutable and synchronized fields
	mu      sync.Mutex // protects the following fields
	refs    uint32     // refcount of acquired handles to the log file
	close   bool       // intent to close the file when refs == 0
	closed  flag       // set when the file has been closed
	removed flag       // set when the file has been removed
}

func newLogFile(fh *os.File, id uint64, ttl uint32, size uint64) *logFile {
	lf := &logFile{fh: fh, id: id, ttl: ttl}
	lf.size.Store(size)
	return lf
}

// performIntents handles any resource cleanup when the ref count reaches zero.
func (l *logFile) performIntents() {
	if l.refs != 0 {
		return
	}
	if l.close && !l.closed.set() {
		_ = l.fh.Close()
	}
}

// Close flags the log file to be closed when the ref count reaches zero.
func (l *logFile) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.close = true
	l.performIntents()
}

// Remove unlinks the file from the filesystem.
func (l *logFile) Remove() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.removed.set() {
		_ = os.Remove(l.fh.Name())
	}
}

// Acquire increases the ref count if the log file is still available (not Closed) and returns
// true if it was able.
func (l *logFile) Acquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.close {
		return false
	}

	l.refs++
	return true
}

// Release decreases the ref count after an Acquire and you are done operating on the log file.
func (l *logFile) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refs--
	l.performIntents()
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
	cfg Config
	mu  sync.Mutex
	lfs map[uint32]*logHeap
}

func newLogCollection(cfg Config) *logCollection {
	return &logCollection{
		cfg: cfg,
		lfs: make(map[uint32]*logHeap),
	}
}

func (l *logCollection) Empty() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, lfh := range l.lfs {
		if lfh.Len() > 0 {
			return false
		}
	}
	return true
}

func (l *logCollection) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for ttl := range l.lfs {
		delete(l.lfs, ttl)
	}
}

func (l *logCollection) Include(lf *logFile) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// if the log is over the max log size, don't include it.
	if lf.size.Load() >= l.cfg.Compaction.MaxLogSize {
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
	s   *Store
	r   *io.SectionReader
	lf  *logFile
	rec Record
}

func newLogReader(s *Store, lf *logFile, rec Record) *Reader {
	return &Reader{
		s:   s,
		r:   io.NewSectionReader(lf.fh, int64(rec.Offset), int64(rec.Length)),
		lf:  lf,
		rec: rec,
	}
}

// Revive attempts to revive a trashed piece.
func (l *Reader) Revive(ctx context.Context) error {
	if !l.Trash() {
		return nil
	}
	return l.s.reviveRecord(ctx, l.lf, l.rec)
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
func (l *Reader) Release() { l.lf.Release() }

// Close is like Release but implements io.Closer. The returned error is always nil.
func (l *Reader) Close() error { l.lf.Release(); return nil }

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
	if store_TestLogSizeAndOffset {
		if size := lf.size.Load(); size != uint64(offset) {
			panic(fmt.Sprintf("log file size=%d and offset=%d mismatch for %s",
				size, offset, lf.fh.Name()))
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
