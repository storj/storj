// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/sync2"
)

//
// read handles
//

// stdMultiReadHandle implements MultiReadHandle for stdin.
type stdMultiReadHandle struct {
	stdin io.Reader
	mu    sync.Mutex
	curr  *stdReadHandle
	done  bool
}

func newStdMultiReadHandle(stdin io.Reader) *stdMultiReadHandle {
	return &stdMultiReadHandle{
		stdin: stdin,
	}
}

func (o *stdMultiReadHandle) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.done = true

	return nil
}

func (o *stdMultiReadHandle) SetOffset(offset int64) error {
	return errs.New("cannot set offset on stdin read handle")
}

func (o *stdMultiReadHandle) NextPart(ctx context.Context, length int64) (ReadHandle, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.done {
		return nil, errs.New("already closed")
	}

	if o.curr != nil {
		if !o.curr.done.Wait(ctx) {
			return nil, ctx.Err()
		}

		o.curr.mu.Lock()
		defer o.curr.mu.Unlock()

		if o.curr.err != nil {
			return nil, o.curr.err
		}
	}

	o.curr = &stdReadHandle{
		stdin: o.stdin,
		len:   length,
	}

	return o.curr, nil
}

func (o *stdMultiReadHandle) Info(ctx context.Context) (*ObjectInfo, error) {
	return &ObjectInfo{ContentLength: -1}, nil
}

// Length returns the size of the object.
func (o *stdMultiReadHandle) Length() int64 {
	return -1
}

// stdReadHandle implements ReadHandle for stdin.
type stdReadHandle struct {
	stdin  io.Reader
	mu     sync.Mutex
	done   sync2.Fence
	err    error
	len    int64
	closed bool
}

func (o *stdReadHandle) Info() ObjectInfo { return ObjectInfo{ContentLength: -1} }

func (o *stdReadHandle) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.closed = true
	o.done.Release()

	return nil
}

func (o *stdReadHandle) Read(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.err != nil {
		return 0, o.err
	} else if o.closed {
		return 0, io.EOF
	}

	if o.len >= 0 && o.len < int64(len(p)) {
		p = p[:o.len]
	}

	n, err := o.stdin.Read(p)
	if o.len > 0 {
		o.len -= int64(n)
	}

	if err != nil && o.err == nil {
		o.err = err
		o.done.Release()
	}

	if o.len == 0 {
		o.closed = true
		o.done.Release()
	}

	return n, err
}

//
// write handles
//

// stdWriteHandle implements WriteHandle for stdouts.
type stdWriteHandle struct {
	stdout io.Writer
}

func newStdWriteHandle(stdout io.Writer) *stdWriteHandle {
	return &stdWriteHandle{
		stdout: stdout,
	}
}

func (s *stdWriteHandle) Write(b []byte) (int, error) { return s.stdout.Write(b) }
func (s *stdWriteHandle) Commit() error               { return nil }
func (s *stdWriteHandle) Abort() error                { return nil }
