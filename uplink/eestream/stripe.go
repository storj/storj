// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	mon = monkit.Package()
)

// StripeReader can read and decodes stripes from a set of readers
type StripeReader struct {
	scheme              ErasureScheme
	cond                *sync.Cond
	readerCount         int
	bufs                map[int]*PieceBuffer
	inbufs              map[int][]byte
	inmap               map[int][]byte
	errmap              map[int]error
	forceErrorDetection bool
}

// NewStripeReader creates a new StripeReader from the given readers, erasure
// scheme and max buffer memory.
func NewStripeReader(log *zap.Logger, rs map[int]io.ReadCloser, es ErasureScheme, mbm int, forceErrorDetection bool) *StripeReader {
	readerCount := len(rs)

	r := &StripeReader{
		scheme:              es,
		cond:                sync.NewCond(&sync.Mutex{}),
		readerCount:         readerCount,
		bufs:                make(map[int]*PieceBuffer, readerCount),
		inbufs:              make(map[int][]byte, readerCount),
		inmap:               make(map[int][]byte, readerCount),
		errmap:              make(map[int]error, readerCount),
		forceErrorDetection: forceErrorDetection,
	}

	bufSize := mbm / readerCount
	bufSize -= bufSize % es.ErasureShareSize()
	if bufSize < es.ErasureShareSize() {
		bufSize = es.ErasureShareSize()
	}

	for i := range rs {
		r.inbufs[i] = make([]byte, es.ErasureShareSize())
		r.bufs[i] = NewPieceBuffer(log, make([]byte, bufSize), es.ErasureShareSize(), r.cond)
		// Kick off a goroutine each reader to be copied into a PieceBuffer.
		go func(r io.Reader, buf *PieceBuffer) {
			_, err := io.Copy(buf, r)
			if err != nil {
				buf.SetError(err)
				return
			}
			buf.SetError(io.EOF)
		}(rs[i], r.bufs[i])
	}

	return r
}

// Close closes the StripeReader and all PieceBuffers.
func (r *StripeReader) Close() error {
	errs := make(chan error, len(r.bufs))
	for _, buf := range r.bufs {
		go func(c io.Closer) {
			errs <- c.Close()
		}(buf)
	}
	var first error
	for range r.bufs {
		err := <-errs
		if err != nil && first == nil {
			first = Error.Wrap(err)
		}
	}
	return first
}

// ReadStripe reads and decodes the num-th stripe and concatenates it to p. The
// return value is the updated byte slice.
func (r *StripeReader) ReadStripe(ctx context.Context, num int64, p []byte) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	for i := range r.inmap {
		delete(r.inmap, i)
	}

	for r.decodeCouldSucceed() {
		nin, nerr := r.readAvailableShares(ctx, num)
		if nin == 0 && nerr == 0 {
			r.cond.Wait()
			continue
		}

		// only attempt to decode if we have enough shares and we just
		// got more in the map.
		if nin > 0 && r.hasEnoughShares() {
			out, err := r.scheme.Decode(p, r.inmap)
			if err != nil {
				if r.shouldWaitForMore() {
					continue
				}
				return nil, err
			}
			return out, nil
		}
	}

	// could not read enough shares to attempt a decode
	return nil, r.combineErrs(num)
}

// readAvailableShares reads the available num-th erasure shares from the piece
// buffers without blocking. The return value n is the number of erasure shares
// read.
func (r *StripeReader) readAvailableShares(ctx context.Context, num int64) (nin, nerr int) {
	defer mon.Task()(&ctx)(nil)

	for i, buf := range r.bufs {
		if r.inmap[i] != nil || r.errmap[i] != nil {
			continue
		}

		if buf.HasShare(num) {
			err := buf.ReadShare(num, r.inbufs[i])
			if err != nil {
				r.errmap[i] = err
				nerr++
			} else {
				r.inmap[i] = r.inbufs[i]
				nin++
			}
		}
	}

	return nin, nerr
}

// necessaryShares returns the number of shares necessary to do a decode including
// the extra shares for any error detection if possible.
func (r *StripeReader) necessaryShares() int {
	if r.forceErrorDetection && r.scheme.RequiredCount() < r.scheme.TotalCount() {
		return r.scheme.RequiredCount() + 1
	}
	return r.scheme.RequiredCount()
}

// decodeCouldSucceed checks if there are any pending readers to get a share from.
func (r *StripeReader) decodeCouldSucceed() (pending bool) {
	remainingReaders := r.readerCount - len(r.errmap) - len(r.inmap)
	remainingShares := r.necessaryShares() - len(r.inmap)
	return remainingReaders >= remainingShares
}

// hasEnoughShares check if there are enough erasure shares read to attempt
// a decode.
func (r *StripeReader) hasEnoughShares() (enough bool) {
	return len(r.inmap) >= r.necessaryShares()
}

// shouldWaitForMore checks if it makes sense to wait for more erasure shares to
// attempt an error correction. it's always worthwhile to acquire more shares if
// because they may give us the redundancy needed to detect and repair it.
func (r *StripeReader) shouldWaitForMore() (ok bool) {
	return r.readerCount > len(r.inmap)+len(r.errmap)
}

// combineErrs makes a useful error message from the errors in errmap.
// combineErrs always returns an error.
func (r *StripeReader) combineErrs(num int64) error {
	if len(r.errmap) == 0 {
		return Error.New("programmer error: no errors to combine")
	}
	errstrings := make([]string, 0, len(r.errmap))
	for i, err := range r.errmap {
		errstrings = append(errstrings, fmt.Sprintf("\nerror retrieving piece %02d: %v", i, err))
	}
	sort.Strings(errstrings)
	return Error.New("failed to download stripe %d: %s", num, strings.Join(errstrings, ""))
}
