// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"context"
	"io"
	"io/ioutil"
	"reflect"

	"github.com/vivint/infectious"

	"storj.io/storj/internal/pkg/readcloser"
	"storj.io/storj/pkg/ranger"
)

type decodedReader struct {
	ctx    context.Context
	cancel context.CancelFunc
	rs     map[int]io.ReadCloser
	es     ErasureScheme
	outbuf []byte
	err    error
	chans  map[int]chan block
	cb     int64 // current block number
	eb     int64 // expected number of blocks
}

type block struct {
	i    int    // reader index in the map
	num  int64  // block number
	data []byte // block data
	err  error  // error reading the block
}

// DecodeReaders takes a map of readers and an ErasureScheme returning a
// combined Reader.
//
// rs is a map of erasure piece numbers to erasure piece streams.
// expectedSize is the number of bytes expected to be returned by the Reader.
// mbm is the maximum memory (in bytes) to be allocated for read buffers. If
// set to 0, the minimum possible memory will be used.
func DecodeReaders(ctx context.Context, rs map[int]io.ReadCloser,
	es ErasureScheme, expectedSize int64, mbm int) io.ReadCloser {
	if expectedSize < 0 {
		return readcloser.FatalReadCloser(Error.New("negative expected size"))
	}
	if expectedSize%int64(es.DecodedBlockSize()) != 0 {
		return readcloser.FatalReadCloser(
			Error.New("expected size not a factor decoded block size"))
	}
	if err := checkMBM(mbm); err != nil {
		return readcloser.FatalReadCloser(err)
	}
	chanSize := mbm / (len(rs) * es.EncodedBlockSize())
	if chanSize < 1 {
		chanSize = 1
	}
	context, cancel := context.WithCancel(ctx)
	dr := &decodedReader{
		ctx:    context,
		cancel: cancel,
		rs:     rs,
		es:     es,
		outbuf: make([]byte, 0, es.DecodedBlockSize()),
		chans:  make(map[int]chan block, len(rs)),
		eb:     expectedSize / int64(es.DecodedBlockSize()),
	}
	// Kick off a goroutine for each reader. Each reads a block from the
	// reader and adds it to a buffered channel. If an error is read
	// (including EOF), a block with the error is added to the channel,
	// the channel is closed and the goroutine exits.
	for i := range rs {
		dr.chans[i] = make(chan block, chanSize)
		go func(i int, ch chan block) {
			// close the channel when the goroutine exits
			defer close(ch)
			for blockNum := int64(0); ; blockNum++ {
				// read the next block
				data := make([]byte, es.EncodedBlockSize())
				_, err := io.ReadFull(dr.rs[i], data)
				// add it to the channel
				select {
				case ch <- block{i, blockNum, data, err}:
					if err != nil {
						// exit the goroutine (will close the channel)
						return
					}
				case <-ctx.Done():
					// Close() has been called
					// exit the goroutine (will close the channel)
					return
				}
			}
		}(i, dr.chans[i])
	}
	return dr
}

func (dr *decodedReader) Read(p []byte) (n int, err error) {
	if len(dr.outbuf) <= 0 {
		// if the output buffer is empty, let's fill it again
		// if we've already had an error, fail
		if dr.err != nil {
			return 0, dr.err
		}
		// return EOF is the expected blocks are read
		if dr.cb >= dr.eb {
			dr.err = io.EOF
			return 0, dr.err
		}
		// read the input buffers of the next block - may also decode it
		inbufs := make(map[int][]byte, len(dr.chans))
		dr.err = dr.readBlock(inbufs)
		if dr.err != nil {
			return 0, dr.err
		}
		// if not decoded yet, decode now what's available
		if len(dr.outbuf) <= 0 {
			dr.outbuf, dr.err = dr.es.Decode(dr.outbuf, inbufs)
			if dr.err != nil {
				return 0, dr.err
			}
		}
		// increment the block counter
		dr.cb++
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
	// close the readers
	var firstErr error
	for _, c := range dr.rs {
		err := c.Close()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (dr *decodedReader) makeSelectCases() []reflect.SelectCase {
	cases := make([]reflect.SelectCase, len(dr.chans)+1)
	// default case for non-blocking selection
	cases[0] = reflect.SelectCase{
		Dir: reflect.SelectDefault, Chan: reflect.Value{}}
	// case for each channel
	for i, ch := range dr.chans {
		cases[i+1] = reflect.SelectCase{
			Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	return cases
}

func (dr *decodedReader) removeCase(
	cases []reflect.SelectCase, index int) []reflect.SelectCase {
	return append(cases[:index], cases[index+1:]...)
}

func (dr *decodedReader) readBlock(inbufs map[int][]byte) error {
	// use reflect to select from the array of channels
	cases := dr.makeSelectCases()
	// iterate until the new block is received from enough channels
	for len(cases) > 1 {
		// non-blocking select - harvest available input buffers
		chosen, value, ok := reflect.Select(cases)
		if chosen == 0 {
			// default case - no more input buffers available
			// required+1 will be enough to decode or detect most errors
			if len(inbufs) >= dr.es.RequiredCount()+1 ||
				len(inbufs) >= dr.es.TotalCount() {
				// we have enough input buffers, fill the decoded output buffer
				var err error
				dr.outbuf, err = dr.es.Decode(dr.outbuf, inbufs)
				if err == nil {
					return nil
				}
				// if error is detected, iterate more to try error correction
				// with more input buffers
				if !infectious.NotEnoughShares.Contains(err) &&
					!infectious.TooManyErrors.Contains(err) {
					return err
				}
			}
			// blocking select - wait for more input buffers
			chosen, value, ok = reflect.Select(cases[1:])
			chosen++
		}
		if !ok {
			// the channel is closed - remove it from further selects
			cases = dr.removeCase(cases, chosen)
			continue
		}
		b := value.Interface().(block)
		if b.err != nil {
			// read error - remove the channel from further selects
			cases = dr.removeCase(cases, chosen)
			continue
		}
		// check if this is the expected block number
		// if not (slow reader), discard it and make another select
		if b.num == dr.cb {
			inbufs[b.i] = b.data
			// remove the channel from further selects to avoid reading
			// the next block if reader is fast
			cases = dr.removeCase(cases, chosen)
		}
	}
	return nil
}

type decodedRanger struct {
	es     ErasureScheme
	rrs    map[int]ranger.RangeCloser
	inSize int64
	mbm    int // max buffer memory
}

// Decode takes a map of Rangers and an ErasureScheme and returns a combined
// Ranger.
//
// rrs is a map of erasure piece numbers to erasure piece rangers.
// mbm is the maximum memory (in bytes) to be allocated for read buffers. If
// set to 0, the minimum possible memory will be used.
func Decode(rrs map[int]ranger.RangeCloser, es ErasureScheme, mbm int) (ranger.RangeCloser, error) {
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
		return ranger.NopCloser(ranger.ByteRanger(nil)), nil
	}
	if size%int64(es.EncodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure decoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	return &decodedRanger{
		es:     es,
		rrs:    rrs,
		inSize: size,
		mbm:    mbm,
	}, nil
}

func (dr *decodedRanger) Size() int64 {
	blocks := dr.inSize / int64(dr.es.EncodedBlockSize())
	return blocks * int64(dr.es.DecodedBlockSize())
}

func (dr *decodedRanger) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	// offset and length might not be block-aligned. figure out which
	// blocks contain this request
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, dr.es.DecodedBlockSize())

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
				firstBlock*int64(dr.es.EncodedBlockSize()),
				blockCount*int64(dr.es.EncodedBlockSize()))
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
	r := DecodeReaders(ctx, readers, dr.es, length, dr.mbm)
	// offset might start a few bytes in, potentially discard the initial bytes
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(dr.es.DecodedBlockSize()))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	// length might not have included all of the blocks, limit what we return
	return readcloser.LimitReadCloser(r, length), nil
}

func (dr *decodedRanger) Close() error {
	errs := make(chan error, len(dr.rrs))
	for _, rr := range dr.rrs {
		go func(rr ranger.RangeCloser) {
			errs <- rr.Close()
		}(rr)
	}
	var first error
	for range dr.rrs {
		err := <-errs
		if err != nil && first == nil {
			first = Error.Wrap(err)
		}
	}
	return first
}
