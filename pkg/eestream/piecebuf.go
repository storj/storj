// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"io"
	"io/ioutil"
	"sync"

	"go.uber.org/zap"
)

// PieceBuffer is a synchronized buffer for storing erasure shares for a piece.
type PieceBuffer struct {
	buf        []byte
	shareSize  int
	cv         *sync.Cond
	cvNewData  *sync.Cond
	rpos, wpos int
	full       bool
	c          int64 // current erasure share number
	err        error
}

// NewPieceBuffer creates and initializes a new PieceBuffer using buf as its
// internal content. If new data is written to the buffer, cvNewData will be
// notified.
func NewPieceBuffer(buf []byte, shareSize int, cvNewData *sync.Cond) *PieceBuffer {
	return &PieceBuffer{
		buf:       buf,
		shareSize: shareSize,
		cv:        sync.NewCond(&sync.Mutex{}),
		cvNewData: cvNewData,
	}
}

// Read reads the next len(p) bytes from the buffer or until the buffer is
// drained. The return value n is the number of bytes read. If the buffer has
// no data to return and no error is set, the call will block until new data is
// written to the buffer. Otherwise the error will be returned.
func (b *PieceBuffer) Read(p []byte) (n int, err error) {
	defer b.cv.Broadcast()
	b.cv.L.Lock()
	defer b.cv.L.Unlock()

	for b.empty() {
		if b.err != nil {
			return 0, b.err
		}
		b.cv.Wait()
	}

	if b.rpos < b.wpos {
		n = copy(p, b.buf[b.rpos:b.wpos])
	} else {
		n = copy(p, b.buf[b.rpos:])
	}
	b.rpos = (b.rpos + n) % len(b.buf)
	b.full = false

	return n, nil
}

// Write writes the contents of p into the buffer. If the buffer is full it
// will block until some data is read from it, or an error is set. The return
// value n is the number of bytes written. If an error was set, it be returned.
func (b *PieceBuffer) Write(p []byte) (n int, err error) {
	for n < len(p) {
		nn, err := b.write(p[n:])
		n += nn
		if err != nil {
			return n, err
		}
		b.notifyNewData()
	}
	return n, nil
}

// write is a helper method that takes care for the locking on each copy
// iteration.
func (b *PieceBuffer) write(p []byte) (n int, err error) {
	defer b.cv.Broadcast()
	b.cv.L.Lock()
	defer b.cv.L.Unlock()

	for b.full {
		if b.err != nil {
			return n, b.err
		}
		b.cv.Wait()
	}

	var wr int
	if b.wpos < b.rpos {
		wr = copy(b.buf[b.wpos:b.rpos], p)
	} else {
		wr = copy(b.buf[b.wpos:], p)
	}

	n += wr
	b.wpos = (b.wpos + wr) % len(b.buf)
	if b.wpos == b.rpos {
		b.full = true
	}

	return n, nil
}

// Close sets io.ErrClosedPipe to the buffer to prevent further writes and
// blocking on read.
func (b *PieceBuffer) Close() error {
	b.SetError(io.ErrClosedPipe)
	return nil
}

// SetError sets an error to be returned by Read and Write. Read will return
// the error after all data is read from the buffer.
func (b *PieceBuffer) SetError(err error) {
	b.setError(err)
	b.notifyNewData()
}

// setError is a helper method that locks the mutex before setting the error.
func (b *PieceBuffer) setError(err error) {
	defer b.cv.Broadcast()
	b.cv.L.Lock()
	defer b.cv.L.Unlock()

	b.err = err
}

// getError is a helper method that locks the mutex before getting the error.
func (b *PieceBuffer) getError() error {
	b.cv.L.Lock()
	defer b.cv.L.Unlock()

	return b.err
}

// notifyNewData notifies cvNewData that new data is written to the buffer.
func (b *PieceBuffer) notifyNewData() {
	b.cvNewData.L.Lock()
	defer b.cvNewData.L.Unlock()

	b.cvNewData.Broadcast()
}

// empty chacks if the buffer is empty.
func (b *PieceBuffer) empty() bool {
	return !b.full && b.rpos == b.wpos
}

// buffered returns the number of bytes that can be read from the buffer
// without blocking.
func (b *PieceBuffer) buffered() int {
	b.cv.L.Lock()
	defer b.cv.L.Unlock()

	switch {
	case b.rpos < b.wpos:
		return b.wpos - b.rpos
	case b.rpos > b.wpos:
		return len(b.buf) + b.wpos - b.rpos
	case b.full:
		return len(b.buf)
	default: // empty
		return 0
	}
}

// HasShare checks if the num-th share can be read from the buffer without
// blocking. If there are older erasure shares in the buffer, they will be
// discarded to leave room for the newer erasure shares to be written.
func (b *PieceBuffer) HasShare(num int64) bool {
	if num < b.c {
		// we should never get here!
		zap.S().Fatalf("Checking for erasure share %d while the current erasure share is %d.", num, b.c)
	}

	if b.getError() != nil {
		return true
	}

	bufShares := int64(b.buffered() / b.shareSize)
	if num-b.c > 0 {
		if bufShares > num-b.c {
			b.discardUntil(num)
		} else {
			b.discardUntil(b.c + bufShares)
		}
		bufShares = int64(b.buffered() / b.shareSize)
	}

	return bufShares > num-b.c
}

// ReadShare reads the num-th erasure share from the buffer into p. Any shares
// before num will be discarded from the buffer.
func (b *PieceBuffer) ReadShare(num int64, p []byte) error {
	if num < b.c {
		// we should never get here!
		zap.S().Fatalf("Trying to read erasure share %d while the current erasure share is already %d.", num, b.c)
	}

	err := b.discardUntil(num)
	if err != nil {
		return err
	}

	_, err = io.ReadFull(b, p)
	if err != nil {
		return err
	}

	b.c++

	return nil
}

// discardUntil discards all erasure shares from the buffer until the num-th
// erasure share exclusively.
func (b *PieceBuffer) discardUntil(num int64) error {
	if num <= b.c {
		return nil
	}

	_, err := io.CopyN(ioutil.Discard, b, (num-b.c)*int64(b.shareSize))
	if err != nil {
		return err
	}

	b.c = num

	return nil
}
