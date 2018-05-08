// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"io"
	"io/ioutil"
	"reflect"
	"strings"

	"storj.io/storj/internal/pkg/readcloser"
	"storj.io/storj/pkg/ranger"
)

type decodedReader struct {
	rs     map[int]io.ReadCloser
	es     ErasureScheme
	outbuf []byte
	err    error
	chans  map[int]chan block
	cb     int // current block number
}

type readerError struct {
	i   int // reader index in the map
	err error
}

type block struct {
	i    int    // reader index in the map
	num  int    // block number
	data []byte // block data
	err  error  // error reading the block
}

// DecodeReaders takes a map of readers and an ErasureScheme returning a
// combined Reader. The map, 'rs', must be a mapping of erasure piece numbers
// to erasure piece streams.
func DecodeReaders(rs map[int]io.ReadCloser, es ErasureScheme) io.ReadCloser {
	dr := &decodedReader{
		rs:     rs,
		es:     es,
		outbuf: make([]byte, 0, es.DecodedBlockSize()),
		chans:  make(map[int]chan block, len(rs)),
	}
	// Kick off a goroutine for each reader. Each reads a block from the
	// reader and adds it to a buffered channel. If an error is read
	// (including EOF), a block with the error is added to the channel,
	// the channel is closed and the gourtine exits.
	// TODO: Ensure that goroutines of slow readers really exit and don't
	// block on adding blocks to the buffered channel.
	for i := range rs {
		dr.chans[i] = make(chan block, 5) // TODO make this configurable
		go func(i int, ch chan block) {
			// close the channel when the goroutine exits
			defer close(ch)
			blockNum := 0
			for {
				// read the next block
				data := make([]byte, es.EncodedBlockSize())
				_, err := io.ReadFull(dr.rs[i], data)
				// add it to the channel
				ch <- block{i, blockNum, data, err}
				if err != nil {
					// exit the goroutine (will close the channel)
					return
				}
				blockNum++
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
			return 0, err
		}
		// Run a selector on the readers' buffered channels
		eofbufs := 0
		inbufs := make(map[int][]byte, len(dr.chans))
		// use reflect to select from the array of channels
		cases := make([]reflect.SelectCase, len(dr.chans)+1)
		// default case for non-blocking selection
		cases[0] = reflect.SelectCase{
			Dir: reflect.SelectDefault, Chan: reflect.Value{}}
		for i, ch := range dr.chans {
			cases[i+1] = reflect.SelectCase{
				Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}
		// iterate until the new block is received from enough channels
		for len(cases) > 1 {
			// non-blocking select - harvest available input buffers
			chosen, value, ok := reflect.Select(cases)
			if chosen == 0 {
				// default case - no more input buffers available, check if we have enough
				// TODO is required+1 enough to detect errors in polynomials of any degree?
				if len(inbufs) >= dr.es.RequiredCount()+1 ||
					len(inbufs) >= dr.es.TotalCount() {
					// we have enough input buffers, fill the decoded output buffer
					dr.outbuf, dr.err = dr.es.Decode(dr.outbuf, inbufs)
					if dr.err == nil {
						break
					}
					// TODO is there better way for error comparision?
					if !strings.Contains(dr.err.Error(), "not enough shares") &&
						!strings.Contains(dr.err.Error(), "too many errors") {
						return 0, dr.err
					}
				}
				// if enough readers are at EOF, return it
				if eofbufs >= dr.es.RequiredCount() {
					dr.err = io.EOF
					return 0, dr.err
				}
				// blocking select - wait for more input buffers
				chosen, value, ok = reflect.Select(cases[1:])
				chosen++
			}
			if !ok {
				// the channel is closed - remove it from further selects
				cases = append(cases[:chosen], cases[chosen+1:]...)
				continue
			}
			b := value.Interface().(block)
			if b.err != nil {
				// remove the channel from further selects
				cases = append(cases[:chosen], cases[chosen+1:]...)
				if b.err == io.EOF {
					// keep track of readers at EOF
					eofbufs++
				}
				continue
			}
			// check if this is the expected block number
			// if not (slow reader), discard it and make another select
			if b.num == dr.cb {
				inbufs[b.i] = b.data
				// remove the channel from further selects to avoid reading
				// the next block if reader is fast
				cases = append(cases[:chosen], cases[chosen+1:]...)
			}
		}
		// if no decoding yet, decode now what's available
		if len(dr.outbuf) <= 0 {
			dr.outbuf, dr.err = dr.es.Decode(dr.outbuf, inbufs)
			if dr.err != nil {
				// if enough readers are at EOF, return it
				if eofbufs >= dr.es.RequiredCount() {
					dr.err = io.EOF
					return 0, dr.err
				}
				// otherwise return the decode error
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
	var firstErr error
	for _, c := range dr.rs {
		err := c.Close()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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

func (dr *decodedRanger) Range(offset, length int64) io.ReadCloser {
	// offset and length might not be block-aligned. figure out which
	// blocks contain this request
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, dr.es.DecodedBlockSize())

	// go ask for ranges for all those block boundaries
	// do it parallel to save from network latency
	readers := make(map[int]io.ReadCloser, len(dr.rrs))
	type indexReadCloser struct {
		i int
		r io.ReadCloser
	}
	result := make(chan indexReadCloser, len(dr.rrs))
	for i, rr := range dr.rrs {
		go func(i int, rr ranger.Ranger) {
			r := rr.Range(
				firstBlock*int64(dr.es.EncodedBlockSize()),
				blockCount*int64(dr.es.EncodedBlockSize()))
			result <- indexReadCloser{i, r}
		}(i, rr)
	}
	// wait for all goroutines to finish and save result in readers map
	for range dr.rrs {
		res := <-result
		readers[res.i] = res.r
	}
	// decode from all those ranges
	r := DecodeReaders(readers, dr.es)
	// offset might start a few bytes in, potentially discard the initial bytes
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(dr.es.DecodedBlockSize()))
	if err != nil {
		return readcloser.FatalReadCloser(Error.Wrap(err))
	}
	// length might not have included all of the blocks, limit what we return
	return readcloser.LimitReadCloser(r, length)
}
