// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"io"
	"io/ioutil"

	"storj.io/storj/pkg/ranger"
)

// A Transformer is a data transformation that may change the size of the blocks
// of data it operates on in a deterministic fashion.
type Transformer interface {
	InBlockSize() int  // The block size prior to transformation
	OutBlockSize() int // The block size after transformation
	Transform(out, in []byte, blockNum int64) ([]byte, error)
}

type transformedReader struct {
	r        io.Reader
	t        Transformer
	blockNum int64
	inbuf    []byte
	outbuf   []byte
}

// TransformReader applies a Transformer to a Reader. startingBlockNum should
// probably be 0 unless you know you're already starting at a block offset.
func TransformReader(r io.Reader, t Transformer,
	startingBlockNum int64) io.Reader {
	return &transformedReader{
		r:        r,
		t:        t,
		blockNum: startingBlockNum,
		inbuf:    make([]byte, t.InBlockSize()),
		outbuf:   make([]byte, 0, t.OutBlockSize()),
	}
}

func (t *transformedReader) Read(p []byte) (n int, err error) {
	if len(t.outbuf) <= 0 {
		// If there's no more buffered data left, let's fill the buffer with
		// the next block
		_, err = io.ReadFull(t.r, t.inbuf)
		if err != nil {
			return 0, err
		}
		t.outbuf, err = t.t.Transform(t.outbuf, t.inbuf, t.blockNum)
		if err != nil {
			return 0, Error.Wrap(err)
		}
		t.blockNum++
	}

	// return as much as we can from the current buffered block
	n = copy(p, t.outbuf)
	// slide the uncopied data to the beginning of the buffer
	copy(t.outbuf, t.outbuf[n:])
	// resize the buffer
	t.outbuf = t.outbuf[:len(t.outbuf)-n]
	return n, nil
}

type transformedRanger struct {
	rr ranger.Ranger
	t  Transformer
}

// Transform will apply a Transformer to a Ranger.
func Transform(rr ranger.Ranger, t Transformer) (ranger.Ranger, error) {
	if rr.Size()%int64(t.InBlockSize()) != 0 {
		return nil, Error.New("invalid transformer and range reader combination." +
			"the range reader size is not a multiple of the block size")
	}
	return &transformedRanger{rr: rr, t: t}, nil
}

func (t *transformedRanger) Size() int64 {
	blocks := t.rr.Size() / int64(t.t.InBlockSize())
	return blocks * int64(t.t.OutBlockSize())
}

// calcEncompassingBlocks is a useful helper function that, given an offset,
// length, and blockSize, will tell you which blocks contain the requested
// offset and length
func calcEncompassingBlocks(offset, length int64, blockSize int) (
	firstBlock, blockCount int64) {
	firstBlock = offset / int64(blockSize)
	if length <= 0 {
		return firstBlock, 0
	}
	lastBlock := (offset + length) / int64(blockSize)
	if (offset+length)%int64(blockSize) == 0 {
		return firstBlock, lastBlock - firstBlock
	}
	return firstBlock, 1 + lastBlock - firstBlock
}

func (t *transformedRanger) Range(offset, length int64) io.Reader {
	// Range may not have been called for block-aligned offsets and lengths, so
	// let's figure out which blocks encompass the request
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, t.t.OutBlockSize())
	// okay, now let's get the range on the underlying ranger for those blocks
	// and then Transform it.
	r := TransformReader(
		t.rr.Range(
			firstBlock*int64(t.t.InBlockSize()),
			blockCount*int64(t.t.InBlockSize())), t.t, firstBlock)
	// the range we got potentially includes more than we wanted. if the
	// offset started past the beginning of the first block, we need to
	// swallow the first few bytes
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(t.t.OutBlockSize()))
	if err != nil {
		if err == io.EOF {
			return bytes.NewReader(nil)
		}
		return ranger.FatalReader(Error.Wrap(err))
	}
	// the range might have been too long. only return what was requested
	return io.LimitReader(r, length)
}
