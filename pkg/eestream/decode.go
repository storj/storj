// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"io"
	"io/ioutil"

	"storj.io/storj/pkg/ranger"
)

type decodedReader struct {
	rs     map[int]io.Reader
	es     ErasureScheme
	inbufs map[int][]byte
	outbuf []byte
	err    error
}

// DecodeReaders takes a map of readers and an ErasureScheme returning a
// combined Reader. The map, 'rs', must be a mapping of erasure piece numbers
// to erasure piece streams.
func DecodeReaders(rs map[int]io.Reader, es ErasureScheme) io.Reader {
	dr := &decodedReader{
		rs:     rs,
		es:     es,
		inbufs: make(map[int][]byte, len(rs)),
		outbuf: make([]byte, 0, es.DecodedBlockSize()),
	}
	for i := range rs {
		dr.inbufs[i] = make([]byte, es.EncodedBlockSize())
	}
	return dr
}

func (dr *decodedReader) Read(p []byte) (n int, err error) {
	if len(dr.outbuf) <= 0 {
		// if the output buffer is empty, let's fill it again
		// if we've already had an error, fail
		if dr.err != nil {
			return 0, err
		}
		// we're going to kick off a bunch of goroutines. make a
		// channel to catch those goroutine errors. importantly,
		// the channel has a buffer size to contain all the errors
		// even if we read none, so we can return without receiving
		// every channel value
		errs := make(chan error, len(dr.rs))
		for i := range dr.rs {
			go func(i int) {
				// fill the buffer from the piece input
				_, err := io.ReadFull(dr.rs[i], dr.inbufs[i])
				errs <- err
			}(i)
		}
		// catch all the errors
		for range dr.rs {
			err := <-errs
			if err != nil {
				// return on the first failure
				dr.err = err
				return 0, err
			}
		}
		// we have all the input buffers, fill the decoded output buffer
		dr.outbuf, err = dr.es.Decode(dr.outbuf, dr.inbufs)
		if err != nil {
			return 0, err
		}
	}

	// copy what data we have to the output
	n = copy(p, dr.outbuf)
	// slide the remaining bytes to the beginning
	copy(dr.outbuf, dr.outbuf[n:])
	// shrink the remaining buffer
	dr.outbuf = dr.outbuf[:len(dr.outbuf)-n]
	return n, nil
}

type decodedRanger struct {
	es     ErasureScheme
	rrs    map[int]ranger.Ranger
	inSize int64
}

// Decode takes a map of Rangers and an ErasureSchema and returns a combined
// Ranger. The map, 'rrs', must be a mapping of erasure piece numbers
// to erasure piece rangers.
func Decode(rrs map[int]ranger.Ranger, es ErasureScheme) (
	ranger.Ranger, error) {
	size := int64(-1)
	for _, rr := range rrs {
		if size == -1 {
			size = rr.Size()
		} else {
			if size != rr.Size() {
				return nil, Error.New("decode failure: range reader sizes don't " +
					"all match")
			}
		}
	}
	if size == -1 {
		return ranger.ByteRanger(nil), nil
	}
	if size%int64(es.EncodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure decoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	if len(rrs) < es.RequiredCount() {
		return nil, Error.New("not enough readers to reconstruct data!")
	}
	return &decodedRanger{
		es:     es,
		rrs:    rrs,
		inSize: size,
	}, nil
}

func (dr *decodedRanger) Size() int64 {
	blocks := dr.inSize / int64(dr.es.EncodedBlockSize())
	return blocks * int64(dr.es.DecodedBlockSize())
}

func (dr *decodedRanger) Range(offset, length int64) io.Reader {
	// offset and length might not be block-aligned. figure out which
	// blocks contain this request
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, dr.es.DecodedBlockSize())

	// go ask for ranges for all those block boundaries
	readers := make(map[int]io.Reader, len(dr.rrs))
	for i, rr := range dr.rrs {
		readers[i] = rr.Range(
			firstBlock*int64(dr.es.EncodedBlockSize()),
			blockCount*int64(dr.es.EncodedBlockSize()))
	}
	// decode from all those ranges
	r := DecodeReaders(readers, dr.es)
	// offset might start a few bytes in, potentially discard the initial bytes
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(dr.es.DecodedBlockSize()))
	if err != nil {
		return ranger.FatalReader(Error.Wrap(err))
	}
	// length might not have included all of the blocks, limit what we return
	return io.LimitReader(r, length)
}
