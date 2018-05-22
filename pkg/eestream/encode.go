// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"io"
	"io/ioutil"
	"time"

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
	r       io.Reader
	es      ErasureScheme
	inbuf   []byte
	outbufs [][]byte
	chans   map[int]chan []byte
	err     error
}

// EncodeReader will take a Reader and an ErasureScheme and return a slice of
// Readers. maxBufferMemory is the maximum memory (in bytes) to be allocated
// for read buffers. If set to 0, the minimum possible memory will be used.
func EncodeReader(r io.Reader, es ErasureScheme, maxBufferMemory int) []io.Reader {
	er := &encodedReader{
		r:       r,
		es:      es,
		inbuf:   make([]byte, es.DecodedBlockSize()),
		outbufs: make([][]byte, es.TotalCount()),
		chans:   make(map[int]chan []byte, es.TotalCount()),
	}
	readers := make([]io.Reader, 0, es.TotalCount())
	for i := 0; i < es.TotalCount(); i++ {
		readers = append(readers, &encodedPiece{
			er: er,
			i:  i,
		})
	}
	if maxBufferMemory < 0 {
		er.err = Error.New("negative max buffer memory")
		return readers
	}
	chanSize := maxBufferMemory / (es.TotalCount() * es.EncodedBlockSize())
	if chanSize < 1 {
		chanSize = 1
	}
	for i := 0; i < es.TotalCount(); i++ {
		er.chans[i] = make(chan []byte, chanSize)
	}
	go er.fillBuffer()
	return readers
}

func (er *encodedReader) fillBuffer() {
	defer er.closeChannels()
	for {
		_, err := io.ReadFull(er.r, er.inbuf)
		if err != nil {
			return
		}
		err = er.es.Encode(er.inbuf, er.addToReader)
		if err != nil {
			return
		}
	}
}

func (er *encodedReader) addToReader(num int, data []byte) {
	if er.chans[num] == nil {
		// this channel is already closed for slowliness - skip it
		return
	}
	// data is reused by infecious, so add a copy to the channel
	tmp := make([]byte, len(data))
	copy(tmp, data)
	// add data copy to the respective reader's channel
	for {
		select {
		case er.chans[num] <- tmp:
			return
		// block for no more than 50 ms
		case <-time.After(50 * time.Millisecond):
			if er.isSlowChannel(num) {
				close(er.chans[num])
				er.chans[num] = nil
				return
			}
		}
	}
}

func (er *encodedReader) isSlowChannel(num int) bool {
	// check how many channels are already empty
	ec := 0
	for i := range er.chans {
		if len(er.chans[i]) == 0 {
			ec++
		}
	}
	// check if more than the required channels are empty,
	// i.e. the current channels is slow and should be closed
	return ec >= er.es.RequiredCount()
}

func (er *encodedReader) closeChannels() {
	for i := range er.chans {
		if er.chans[i] != nil {
			close(er.chans[i])
		}
	}
}

type encodedPiece struct {
	er *encodedReader
	i  int
}

func (ep *encodedPiece) Read(p []byte) (n int, err error) {
	if ep.er.err != nil {
		return 0, ep.er.err
	}
	outbufs, i := ep.er.outbufs, ep.i
	if len(outbufs[i]) <= 0 {
		// take the next block from the cannel or block if channel is empty
		var ok bool
		outbufs[i], ok = <-ep.er.chans[i]
		if !ok {
			// channel is closed
			// TODO should be different error if channel closed for slowliness
			// which is better: io.ErrUnexpectedEOF or io.ErrClosedPipe?
			return 0, io.EOF
		}
	}

	// we have some buffer remaining for this piece. write it to the output
	n = copy(p, outbufs[i])
	// slide the unused (if any) bytes to the beginning of the buffer
	copy(outbufs[i], outbufs[i][n:])
	// and shrink the buffer
	outbufs[i] = outbufs[i][:len(outbufs[i])-n]
	return n, nil
}

// EncodedRanger will take an existing Ranger and provide a means to get
// multiple Ranged sub-Readers. EncodedRanger does not match the normal Ranger
// interface.
type EncodedRanger struct {
	es              ErasureScheme
	rr              ranger.Ranger
	maxBufferMemory int
}

// NewEncodedRanger creates an EncodedRanger
func NewEncodedRanger(rr ranger.Ranger, es ErasureScheme,
	maxBufferMemory int) (*EncodedRanger, error) {
	if rr.Size()%int64(es.DecodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	if maxBufferMemory < 0 {
		return nil, Error.New("negative max buffer memory")
	}
	return &EncodedRanger{
		es:              es,
		rr:              rr,
		maxBufferMemory: maxBufferMemory,
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
		blockCount*int64(er.es.DecodedBlockSize())), er.es, er.maxBufferMemory)

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
