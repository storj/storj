// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"io"
	"math"
	"os"
	"sync"

	"github.com/zeebo/errs"
)

type logFile struct {
	// immutable fields
	fh *os.File
	id uint32

	// mutable but unsynchronized fields
	size uint64

	// mutable and synchronized fields
	mu      sync.Mutex // protects the following fields
	refs    uint32     // refcount of acquired handles to the log file
	close   bool       // intent to close the file when refs == 0
	closed  flag       // set when the file has been closed
	removed flag       // set when the file has been removed
}

func newLogFile(fh *os.File, id uint32, size uint64) *logFile {
	return &logFile{
		fh:   fh,
		id:   id,
		size: size,
	}
}

func (l *logFile) performIntents() {
	if l.refs != 0 {
		return
	}
	if l.close && !l.closed.set() {
		_ = l.fh.Close()
	}
}

func (l *logFile) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.close = true
	l.performIntents()
}

func (l *logFile) Remove() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.removed.set() {
		_ = os.Remove(l.fh.Name())
	}
}

func (l *logFile) Acquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.close {
		return false
	}

	l.refs++
	return true
}

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
func (h logHeap) Less(i, j int) bool { return h[i].size < h[j].size }
func (h logHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *logHeap) Push(x any)        { *h = append(*h, x.(*logFile)) }
func (h *logHeap) Pop() any {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[:n-1]
	return x
}

//
// Reader
//

// Reader is a type that reads a section from a log file.
type Reader struct {
	r   *io.SectionReader
	lf  *logFile
	rec record
}

func newLogReader(lf *logFile, rec record) *Reader {
	return &Reader{
		r:   io.NewSectionReader(lf.fh, int64(rec.offset), int64(rec.length)),
		lf:  lf,
		rec: rec,
	}
}

// Key returns the key of thereader.
func (l *Reader) Key() Key { return l.rec.key }

// Size returns the size of the reader.
func (l *Reader) Size() int64 { return int64(l.rec.length) }

// Trash returns true if the reader was for a trashed piece.
func (l *Reader) Trash() bool { return l.rec.expires.trash() }

// Seek implements io.Seeker.
func (l *Reader) Seek(offset int64, whence int) (int64, error) {
	return l.r.Seek(offset, whence)
}

// ReadAt implements io.ReaderAt.
func (l *Reader) ReadAt(p []byte, off int64) (int, error) {
	return l.r.ReadAt(p, off)
}

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
	store *store
	lf    *logFile

	mu       sync.Mutex // protects the following fields
	canceled flag
	closed   flag
	rec      record
}

func (h *Writer) Size() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return int64(h.rec.length)
}

// Close commits the writes that have happened. Close or Cancel must be called at least once.
func (h *Writer) Close() error {
	// if we are not the first to close or we are canceled, do nothing
	h.mu.Lock()
	if h.closed.set() || h.canceled.get() {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()

	err := h.store.addRecord(h.rec)
	h.lf.size += uint64(h.rec.length)
	h.store.replaceLogFile(h.lf)

	return err
}

// Cancel discards the writes that have happened. Close or Cancel must be called at least once.
func (h *Writer) Cancel() {
	// if we are not the first to cancel or we are closed, do nothing
	h.mu.Lock()
	if h.canceled.set() || h.closed.get() {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()

	// attempt to seek backwards the amount we have written to reclaim space
	if h.rec.length != 0 {
		_, _ = h.lf.fh.Seek(-int64(h.rec.length), io.SeekCurrent)
	}
	h.store.replaceLogFile(h.lf)
}

// Write implements io.Writer.
func (h *Writer) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.canceled || h.closed {
		return 0, errs.New("invalid handle")
	} else if uint64(h.rec.length)+uint64(len(p)) > math.MaxUint32 {
		return 0, errs.New("piece too large")
	}

	n, err = h.lf.fh.Write(p)
	h.rec.length += uint32(n)
	return n, err
}
