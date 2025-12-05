// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"
)

//
// read handles
//

// GenericReader is an interface that can be turned into a GenericMultiReadHandle.
type GenericReader interface {
	io.Closer
	io.ReaderAt
}

// NewGenericMultiReadHandle implements MultiReadHandle for *os.Files.
func NewGenericMultiReadHandle(r GenericReader, info ObjectInfo) *GenericMultiReadHandle {
	return &GenericMultiReadHandle{
		r:    r,
		info: info,
	}
}

// GenericMultiReadHandle can turn any GenericReader into a MultiReadHandle.
type GenericMultiReadHandle struct {
	r    GenericReader
	info ObjectInfo

	mu   sync.Mutex
	off  int64
	done bool
}

// Close closes the GenericMultiReadHandle.
func (o *GenericMultiReadHandle) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return nil
	}
	o.done = true

	return o.r.Close()
}

// SetOffset will set the offset for the next call to NextPart.
func (o *GenericMultiReadHandle) SetOffset(offset int64) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return errs.New("already closed")
	}

	o.off = offset
	return nil
}

// NextPart returns a ReadHandle of length bytes at the current offset.
func (o *GenericMultiReadHandle) NextPart(ctx context.Context, length int64) (ReadHandle, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return nil, errs.New("already closed")
	}

	if o.off < 0 {
		o.off += o.info.ContentLength
	}
	if o.off < 0 || o.off > o.info.ContentLength {
		return nil, errs.New("invalid offset: %d for length %d", o.off, o.info.ContentLength)
	}
	if o.off == o.info.ContentLength {
		return nil, io.EOF
	}
	if length < 0 {
		length = o.info.ContentLength
	}
	if o.off+length > o.info.ContentLength {
		length = o.info.ContentLength - o.off
	}

	r := &genericReadHandle{
		r:    o.r,
		info: o.info,
		off:  o.off,
		len:  length,
	}
	o.off += length

	return r, nil
}

// Info returns the object info.
func (o *GenericMultiReadHandle) Info(ctx context.Context) (*ObjectInfo, error) {
	info := o.info
	return &info, nil
}

// Length returns the size of the object.
func (o *GenericMultiReadHandle) Length() int64 {
	return o.info.ContentLength
}

type genericReadHandle struct {
	r    GenericReader
	info ObjectInfo
	off  int64
	len  int64
}

func (o *genericReadHandle) Close() error     { return nil }
func (o *genericReadHandle) Info() ObjectInfo { return o.info }

func (o *genericReadHandle) Read(p []byte) (int, error) {
	if o.len <= 0 {
		return 0, io.EOF
	} else if o.len < int64(len(p)) {
		p = p[:o.len]
	}
	n, err := o.r.ReadAt(p, o.off)
	o.off += int64(n)
	o.len -= int64(n)
	return n, err
}

//
// write handles
//

// GenericWriter is an interface that can be turned into a GenericMultiWriteHandle.
type GenericWriter interface {
	io.WriterAt
	Commit() error
	Abort() error
}

// GenericMultiWriteHandle implements MultiWriteHandle for *os.Files.
type GenericMultiWriteHandle struct {
	w GenericWriter

	mu    sync.Mutex
	off   int64
	tail  bool
	done  bool
	abort bool
}

// NewGenericMultiWriteHandle constructs an *GenericMultiWriteHandle from a GenericWriter.
func NewGenericMultiWriteHandle(w GenericWriter) *GenericMultiWriteHandle {
	return &GenericMultiWriteHandle{
		w: w,
	}
}

func (o *GenericMultiWriteHandle) childAbort() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.done {
		o.abort = true
	}
}

func (o *GenericMultiWriteHandle) status() (done, abort bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.done, o.abort
}

// NextPart returns a WriteHandle expecting length bytes to be written to it.
func (o *GenericMultiWriteHandle) NextPart(ctx context.Context, length int64) (WriteHandle, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return nil, errs.New("already closed")
	} else if o.tail {
		return nil, errs.New("unable to make part after tail part")
	}

	w := &genericWriteHandle{
		parent: o,
		w:      o.w,
		off:    o.off,
		tail:   length < 0,
		len:    length,
	}

	if w.tail {
		o.tail = true
	} else {
		o.off += length
	}

	return w, nil
}

// Commit commits the overall GenericMultiWriteHandle. It errors if
// any parts were aborted.
func (o *GenericMultiWriteHandle) Commit(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return nil
	}
	o.done = true

	if o.abort {
		return errs.Combine(
			errs.New("commit failed: not every child was committed"),
			o.w.Abort(),
		)
	}

	return o.w.Commit()
}

// Abort aborts the overall GenericMultiWriteHandle.
func (o *GenericMultiWriteHandle) Abort(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return nil
	}
	o.done = true
	o.abort = true

	return o.w.Abort()
}

type genericWriteHandle struct {
	parent *GenericMultiWriteHandle
	w      GenericWriter
	done   bool
	off    int64
	tail   bool
	len    int64
}

func (o *genericWriteHandle) Write(p []byte) (int, error) {
	if !o.tail {
		if o.len <= 0 {
			return 0, errs.New("write past maximum length")
		} else if o.len < int64(len(p)) {
			p = p[:o.len]
		}
	}
	n, err := o.w.WriteAt(p, o.off)
	o.off += int64(n)
	if !o.tail {
		o.len -= int64(n)
	}
	return n, err
}

func (o *genericWriteHandle) Commit() error {
	if o.done {
		return nil
	}
	o.done = true

	done, abort := o.parent.status()
	if abort {
		return errs.New("commit failed: parent write handle aborted")
	} else if done {
		return errs.New("commit failed: parent write handle done")
	}
	return nil
}

func (o *genericWriteHandle) Abort() error {
	if o.done {
		return nil
	}
	o.done = true

	o.parent.childAbort()
	return nil
}
