// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"context"
	"io"
	"io/ioutil"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/readcloser"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/ranger"
)

type decodedReader struct {
	ctx             context.Context
	cancel          context.CancelFunc
	readers         map[int]io.ReadCloser
	scheme          ErasureScheme
	stripeReader    *StripeReader
	outbuf          []byte
	err             error
	currentStripe   int64
	expectedStripes int64
	close           sync.Once
	closeErr        error
}

// DecodeReaders takes a map of readers and an ErasureScheme returning a
// combined Reader.
//
// rs is a map of erasure piece numbers to erasure piece streams.
// expectedSize is the number of bytes expected to be returned by the Reader.
// mbm is the maximum memory (in bytes) to be allocated for read buffers. If
// set to 0, the minimum possible memory will be used.
func DecodeReaders(ctx context.Context, rs map[int]io.ReadCloser, es ErasureScheme, expectedSize int64, mbm int) io.ReadCloser {
	if expectedSize < 0 {
		return readcloser.FatalReadCloser(Error.New("negative expected size"))
	}
	if expectedSize%int64(es.StripeSize()) != 0 {
		return readcloser.FatalReadCloser(
			Error.New("expected size (%d) not a factor decoded block size (%d)",
				expectedSize, es.StripeSize()))
	}
	if err := checkMBM(mbm); err != nil {
		return readcloser.FatalReadCloser(err)
	}
	dr := &decodedReader{
		readers:         rs,
		scheme:          es,
		stripeReader:    NewStripeReader(rs, es, mbm),
		outbuf:          make([]byte, 0, es.StripeSize()),
		expectedStripes: expectedSize / int64(es.StripeSize()),
	}
	dr.ctx, dr.cancel = context.WithCancel(ctx)
	// Kick off a goroutine to watch for context cancelation.
	go func() {
		<-dr.ctx.Done()
		_ = dr.Close()
	}()
	return dr
}

func (dr *decodedReader) Read(p []byte) (n int, err error) {
	if len(dr.outbuf) <= 0 {
		// if the output buffer is empty, let's fill it again
		// if we've already had an error, fail
		if dr.err != nil {
			return 0, dr.err
		}
		// return EOF is the expected stripes were read
		if dr.currentStripe >= dr.expectedStripes {
			dr.err = io.EOF
			return 0, dr.err
		}
		// read the input buffers of the next stripe - may also decode it
		dr.outbuf, dr.err = dr.stripeReader.ReadStripe(dr.currentStripe, dr.outbuf)
		if dr.err != nil {
			return 0, dr.err
		}
		dr.currentStripe++
	}

	// copy what data we have to the output
	n = copy(p, dr.outbuf)
	// slide the remaining bytes to the beginning
	copy(dr.outbuf, dr.outbuf[n:])
	// shrink the remaining buffer
	dr.outbuf = dr.outbuf[:len(dr.outbuf)-n]
	return n, nil
}

func (dr *decodedReader) Close() error {
	// cancel the context to terminate reader goroutines
	dr.cancel()
	// avoid double close of readers
	dr.close.Do(func() {
		var errlist errs.Group
		// close the readers
		for _, r := range dr.readers {
			errlist.Add(r.Close())
		}
		// close the stripe reader
		errlist.Add(dr.stripeReader.Close())
		dr.closeErr = errlist.Err()
	})
	return dr.closeErr
}

type decodedRanger struct {
	es     ErasureScheme
	rrs    map[int]ranger.Ranger
	inSize int64
	mbm    int // max buffer memory
}

// Decode takes a map of Rangers and an ErasureScheme and returns a combined
// Ranger.
//
// rrs is a map of erasure piece numbers to erasure piece rangers.
// mbm is the maximum memory (in bytes) to be allocated for read buffers. If
// set to 0, the minimum possible memory will be used.
func Decode(rrs map[int]ranger.Ranger, es ErasureScheme, mbm int) (ranger.Ranger, error) {
	if err := checkMBM(mbm); err != nil {
		return nil, err
	}
	if len(rrs) < es.RequiredCount() {
		return nil, Error.New("not enough readers to reconstruct data!")
	}
	size := int64(-1)
	for _, rr := range rrs {
		if size == -1 {
			size = rr.Size()
		} else {
			if size != rr.Size() {
				return nil, Error.New(
					"decode failure: range reader sizes don't all match")
			}
		}
	}
	if size == -1 {
		return ranger.ByteRanger(nil), nil
	}
	if size%int64(es.ErasureShareSize()) != 0 {
		return nil, Error.New("invalid erasure decoder and range reader combo. "+
			"range reader size (%d) must be a multiple of erasure encoder block size (%d)",
			size, es.ErasureShareSize())
	}
	return &decodedRanger{
		es:     es,
		rrs:    rrs,
		inSize: size,
		mbm:    mbm,
	}, nil
}

func (dr *decodedRanger) Size() int64 {
	blocks := dr.inSize / int64(dr.es.ErasureShareSize())
	return blocks * int64(dr.es.StripeSize())
}

func (dr *decodedRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	// offset and length might not be block-aligned. figure out which
	// blocks contain this request
	firstBlock, blockCount := encryption.CalcEncompassingBlocks(offset, length, dr.es.StripeSize())
	// go ask for ranges for all those block boundaries
	// do it parallel to save from network latency
	readers := make(map[int]io.ReadCloser, len(dr.rrs))
	type indexReadCloser struct {
		i   int
		r   io.ReadCloser
		err error
	}
	result := make(chan indexReadCloser, len(dr.rrs))
	for i, rr := range dr.rrs {
		go func(i int, rr ranger.Ranger) {
			r, err := rr.Range(ctx,
				firstBlock*int64(dr.es.ErasureShareSize()),
				blockCount*int64(dr.es.ErasureShareSize()))
			result <- indexReadCloser{i: i, r: r, err: err}
		}(i, rr)
	}
	// wait for all goroutines to finish and save result in readers map
	for range dr.rrs {
		res := <-result
		if res.err != nil {
			readers[res.i] = readcloser.FatalReadCloser(res.err)
		} else {
			readers[res.i] = res.r
		}
	}
	// decode from all those ranges
	r := DecodeReaders(ctx, readers, dr.es, blockCount*int64(dr.es.StripeSize()), dr.mbm)
	// offset might start a few bytes in, potentially discard the initial bytes
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(dr.es.StripeSize()))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	// length might not have included all of the blocks, limit what we return
	return readcloser.LimitReadCloser(r, length), nil
}

func checkMBM(mbm int) error {
	if mbm < 0 {
		return Error.New("negative max buffer memory")
	}
	return nil
}
