// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"container/heap"
	"context"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"
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

	for ttl := range l.lfs {
		delete(l.lfs, ttl)
	}
}

func (l *logCollection) Include(lf *logFile) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// if the log is over the max log size, don't include it.
	if lf.size.Load() >= compaction_MaxLogSize {
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
	r   *io.SectionReader
	lf  *logFile
	rec Record
}

func newLogReader(lf *logFile, rec Record) *Reader {
	return &Reader{
		r:   io.NewSectionReader(lf.fh, int64(rec.Offset), int64(rec.Length)),
		lf:  lf,
		rec: rec,
	}
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
	ctx    context.Context
	store  *Store
	lf     *logFile
	manual bool

	mu       sync.Mutex // protects the following fields
	canceled flag
	closed   flag
	rec      Record
}

func newAutomaticWriter(ctx context.Context, s *Store, lf *logFile, rec Record) *Writer {
	return &Writer{
		ctx:    ctx,
		store:  s,
		lf:     lf,
		manual: false,

		rec: rec,
	}
}

func newManualWriter(ctx context.Context, s *Store, lf *logFile, rec Record) *Writer {
	return &Writer{
		ctx:    ctx,
		store:  s,
		lf:     lf,
		manual: true,

		rec: rec,
	}
}

// Size returns the number of bytes written to the Writer.
func (h *Writer) Size() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return int64(h.rec.Length)
}

func (h *Writer) done() {
	// always replace the log file.
	h.store.lfc.Include(h.lf)

	// if we are not in manual mode, then we need to automatically unlock.
	if !h.manual {
		h.store.activeMu.RUnlock()
	}
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

	// always do cleanup.
	defer h.done()

	// load the size once. no one else can be mutating the log file at the same time so this is
	// safe.
	size := h.lf.size.Load()

	// we're about to write rSize bytes. if we can align the file to 4k after writing the record by
	// writing less than 64 bytes, try to do so. we do this write separately from appending the
	// record because otherwise we would have to allocate a variable width buffer causing an
	// allocation on every Close instead of just on the calls that fix alignment.
	var padding int
	if align := 4096 - ((uint64(h.rec.Length) + size + RecordSize) % 4096); align > 0 && align < 64 {
		padding, _ = h.lf.fh.Write(make([]byte, align))
	}

	// append the record to the log file for reconstruction.
	var buf [RecordSize]byte
	h.rec.WriteTo(&buf)

	if _, err := h.lf.fh.Write(buf[:]); err != nil {
		// if we can't write the entry, we should abort the write operation so that we can always
		// reconstruct the table from the log file. attempt to reclaim space by seeking backwards
		// to the record offset.
		_, _ = h.lf.fh.Seek(int64(size), io.SeekStart)
		return Error.Wrap(err)
	}

	// if we are not in manual mode, then we need to add the record.
	if !h.manual {
		if err := h.store.addRecord(ctx, h.rec); err != nil {
			// if we can't add the record, we should abort the write operation and attempt to
			// reclaim space by seeking backwards to the record offset.
			_, _ = h.lf.fh.Seek(int64(size), io.SeekStart)
			return Error.Wrap(err)
		}
	}

	// increase our in-memory estimate of the size of the log file for sorting.
	h.lf.size.Add(uint64(h.rec.Length) + uint64(padding) + RecordSize)

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

	// always do cleanup.
	defer h.done()

	// attempt to seek backwards the amount we have written to reclaim space.
	if h.rec.Length != 0 {
		_, _ = h.lf.fh.Seek(-int64(h.rec.Length), io.SeekCurrent)
	}
}

// Write implements io.Writer.
func (h *Writer) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.canceled || h.closed {
		return 0, Error.New("invalid handle")
	} else if uint64(h.rec.Length)+uint64(len(p)) > math.MaxUint32 {
		return 0, Error.New("piece too large")
	}

	n, err = h.lf.fh.Write(p)
	h.rec.Length += uint32(n)

	return n, err
}
