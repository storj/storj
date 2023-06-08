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

// stdMultiWriteHandle implements MultiWriteHandle for stdouts.
type stdMultiWriteHandle struct {
	stdout closableWriter

	mu   sync.Mutex
	next *sync.Mutex
	tail bool
	done bool
}

func newStdMultiWriteHandle(stdout io.Writer) *stdMultiWriteHandle {
	return &stdMultiWriteHandle{
		stdout: closableWriter{Writer: stdout},
		next:   new(sync.Mutex),
	}
}

func (s *stdMultiWriteHandle) NextPart(ctx context.Context, length int64) (WriteHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return nil, errs.New("already closed")
	} else if s.tail {
		return nil, errs.New("unable to make part after tail part")
	}

	next := new(sync.Mutex)
	next.Lock()

	w := &stdWriteHandle{
		stdout: &s.stdout,
		mu:     s.next,
		next:   next,
		tail:   length < 0,
		len:    length,
	}

	s.tail = w.tail
	s.next = next

	return w, nil
}

func (s *stdMultiWriteHandle) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.done = true
	return nil
}

func (s *stdMultiWriteHandle) Abort(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.done = true
	return nil
}

// stdWriteHandle implements WriteHandle for stdouts.
type stdWriteHandle struct {
	stdout *closableWriter
	mu     *sync.Mutex
	next   *sync.Mutex
	tail   bool
	len    int64
}

func (s *stdWriteHandle) unlockNext(err error) {
	if s.next != nil {
		if err != nil {
			s.stdout.close(err)
		}
		s.next.Unlock()
		s.next = nil
	}
}

func (s *stdWriteHandle) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.tail {
		if s.len <= 0 {
			return 0, errs.New("write past maximum length")
		} else if s.len < int64(len(p)) {
			p = p[:s.len]
		}
	}

	n, err := s.stdout.Write(p)

	if !s.tail {
		s.len -= int64(n)
		if s.len == 0 {
			s.unlockNext(err)
		}
	}

	return n, err
}

func (s *stdWriteHandle) Commit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.len = 0
	s.unlockNext(nil)

	return nil
}

func (s *stdWriteHandle) Abort() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.len = 0
	s.unlockNext(context.Canceled)

	return nil
}

type closableWriter struct {
	io.Writer
	err error
}

func (out *closableWriter) Write(p []byte) (int, error) {
	if out.err != nil {
		return 0, out.err
	}
	n, err := out.Writer.Write(p)
	out.err = err
	return n, err
}

func (out *closableWriter) close(err error) {
	out.err = err
}
