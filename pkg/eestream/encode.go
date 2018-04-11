// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"io"
	"io/ioutil"
	"sync"

	"storj.io/storj/pkg/ranger"
)

// ErasureScheme represents the general format of any erasure scheme algorithm.
// If this interface can be implemented, the rest of this library will work
// with it.
type ErasureScheme interface {
	// Encode will take 'in' and call 'out' with erasure coded pieces.
	Encode(in []byte, out func(num int, data []byte)) error

	// Decode will take a mapping of available erasure coded piece num -> data,
	// 'in', and append the combined data to 'out', returning it.
	Decode(out []byte, in map[int][]byte) ([]byte, error)

	// EncodedBlockSize is the size the erasure coded pieces should be that come
	// from Encode and are passed to Decode.
	EncodedBlockSize() int

	// DecodedBlockSize is the size the combined file blocks that should be
	// passed in to Encode and will come from Decode.
	DecodedBlockSize() int

	// Encode will generate this many pieces
	TotalCount() int

	// Decode requires at least this many pieces
	RequiredCount() int
}

type encodedReader struct {
	r               io.Reader
	es              ErasureScheme
	cv              *sync.Cond
	inbuf           []byte
	outbufs         [][]byte
	piecesRemaining int
	err             error
}

// EncodeReader will take a Reader and an ErasureScheme and return a slice of
// Readers
func EncodeReader(r io.Reader, es ErasureScheme) []io.Reader {
	er := &encodedReader{
		r:       r,
		es:      es,
		cv:      sync.NewCond(&sync.Mutex{}),
		inbuf:   make([]byte, es.DecodedBlockSize()),
		outbufs: make([][]byte, es.TotalCount()),
	}
	readers := make([]io.Reader, 0, es.TotalCount())
	for i := 0; i < es.TotalCount(); i++ {
		er.outbufs[i] = make([]byte, 0, es.EncodedBlockSize())
		readers = append(readers, &encodedPiece{
			er: er,
			i:  i,
		})
	}
	return readers
}

func (er *encodedReader) wait() (err error) {
	// have we already failed? just return that
	if er.err != nil {
		return er.err
	}
	// are other pieces still using buffer? wait on a condition variable for
	// the last remaining piece to fill all the buffers.
	if er.piecesRemaining > 0 {
		er.cv.Wait()
		// whoever broadcast a wakeup either set an error or filled the buffers.
		// er.err might be nil, which means the buffers are filled.
		return er.err
	}

	// we are going to set an error or fill the buffers
	defer er.cv.Broadcast()
	defer func() {
		// at the end of this function, if we're returning an error, set er.err
		if err != nil {
			er.err = err
		}
	}()
	_, err = io.ReadFull(er.r, er.inbuf)
	if err != nil {
		return err
	}
	err = er.es.Encode(er.inbuf, func(num int, data []byte) {
		er.outbufs[num] = append(er.outbufs[num], data...)
	})
	if err != nil {
		return err
	}
	// reset piecesRemaining
	er.piecesRemaining = er.es.TotalCount()
	return nil
}

type encodedPiece struct {
	er *encodedReader
	i  int
}

func (ep *encodedPiece) Read(p []byte) (n int, err error) {
	// lock! threadsafety matters here
	ep.er.cv.L.Lock()
	defer ep.er.cv.L.Unlock()

	outbufs, i := ep.er.outbufs, ep.i
	if len(outbufs[i]) <= 0 {
		// if we don't have any buffered result yet, wait until we do
		err := ep.er.wait()
		if err != nil {
			return 0, err
		}
	}

	// we have some buffer remaining for this piece. write it to the output
	n = copy(p, outbufs[i])
	// slide the unused (if any) bytes to the beginning of the buffer
	copy(outbufs[i], outbufs[i][n:])
	// and shrink the buffer
	outbufs[i] = outbufs[i][:len(outbufs[i])-n]
	// if there's nothing left, decrement the amount of pieces we have
	if len(outbufs[i]) <= 0 {
		ep.er.piecesRemaining--
	}
	return n, nil
}

// EncodedRanger will take an existing Ranger and provide a means to get
// multiple Ranged sub-Readers. EncodedRanger does not match the normal Ranger
// interface.
type EncodedRanger struct {
	es ErasureScheme
	rr ranger.Ranger
}

// NewEncodedRanger creates an EncodedRanger
func NewEncodedRanger(rr ranger.Ranger, es ErasureScheme) (*EncodedRanger,
	error) {
	if rr.Size()%int64(es.DecodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	return &EncodedRanger{
		es: es,
		rr: rr,
	}, nil
}

// OutputSize is like Ranger.Size but returns the Size of the erasure encoded
// pieces that come out.
func (er *EncodedRanger) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.es.DecodedBlockSize())
	return blocks * int64(er.es.EncodedBlockSize())
}

// Range is like Ranger.Range, but returns a slice of Readers
func (er *EncodedRanger) Range(offset, length int64) ([]io.Reader, error) {
	// the offset and length given may not be block-aligned, so let's figure
	// out which blocks contain the request.
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, er.es.EncodedBlockSize())
	// okay, now let's encode the reader for the range containing the blocks
	readers := EncodeReader(er.rr.Range(
		firstBlock*int64(er.es.DecodedBlockSize()),
		blockCount*int64(er.es.DecodedBlockSize())), er.es)

	for i, r := range readers {
		// the offset might start a few bytes in, so we potentially have to
		// discard the beginning bytes
		_, err := io.CopyN(ioutil.Discard, r,
			offset-firstBlock*int64(er.es.EncodedBlockSize()))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		// the length might be shorter than a multiple of the block size, so
		// limit it
		readers[i] = io.LimitReader(r, length)
	}
	return readers, nil
}
