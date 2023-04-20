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
