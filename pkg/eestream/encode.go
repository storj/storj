// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"context"
	"io"
	"io/ioutil"
	"sync/atomic"

	"go.uber.org/zap"

	"storj.io/storj/internal/readcloser"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/ranger"
)

// ErasureScheme represents the general format of any erasure scheme algorithm.
// If this interface can be implemented, the rest of this library will work
// with it.
type ErasureScheme interface {
	// Encode will take 'in' and call 'out' with erasure coded pieces.
	Encode(in []byte, out func(num int, data []byte)) error

	// EncodeSingle will take 'in' with the stripe and fill 'out' with the erasure share for piece 'num'.
	EncodeSingle(in, out []byte, num int) error

	// Decode will take a mapping of available erasure coded piece num -> data,
	// 'in', and append the combined data to 'out', returning it.
	Decode(out []byte, in map[int][]byte) ([]byte, error)

	// ErasureShareSize is the size of the erasure shares that come from Encode
	// and are passed to Decode.
	ErasureShareSize() int

	// StripeSize is the size the stripes that are passed to Encode and come
	// from Decode.
	StripeSize() int

	// Encode will generate this many pieces
	TotalCount() int

	// Decode requires at least this many pieces
	RequiredCount() int
}

// RedundancyStrategy is an ErasureScheme with a repair and optimal thresholds
type RedundancyStrategy struct {
	ErasureScheme
	repairThreshold  int
	optimalThreshold int
}

// NewRedundancyStrategy from the given ErasureScheme, repair and optimal thresholds.
//
// repairThreshold is the minimum repair threshold.
// If set to 0, it will be reset to the TotalCount of the ErasureScheme.
// optimalThreshold is the optimal threshold.
// If set to 0, it will be reset to the TotalCount of the ErasureScheme.
func NewRedundancyStrategy(es ErasureScheme, repairThreshold, optimalThreshold int) (RedundancyStrategy, error) {
	if repairThreshold == 0 {
		repairThreshold = es.TotalCount()
	}

	if optimalThreshold == 0 {
		optimalThreshold = es.TotalCount()
	}
	if repairThreshold < 0 {
		return RedundancyStrategy{}, Error.New("negative repair threshold")
	}
	if repairThreshold > 0 && repairThreshold < es.RequiredCount() {
		return RedundancyStrategy{}, Error.New("repair threshold less than required count")
	}
	if repairThreshold > es.TotalCount() {
		return RedundancyStrategy{}, Error.New("repair threshold greater than total count")
	}
	if optimalThreshold < 0 {
		return RedundancyStrategy{}, Error.New("negative optimal threshold")
	}
	if optimalThreshold > 0 && optimalThreshold < es.RequiredCount() {
		return RedundancyStrategy{}, Error.New("optimal threshold less than required count")
	}
	if optimalThreshold > es.TotalCount() {
		return RedundancyStrategy{}, Error.New("optimal threshold greater than total count")
	}
	if repairThreshold > optimalThreshold {
		return RedundancyStrategy{}, Error.New("repair threshold greater than optimal threshold")
	}
	return RedundancyStrategy{ErasureScheme: es, repairThreshold: repairThreshold, optimalThreshold: optimalThreshold}, nil
}

// RepairThreshold is the number of available erasure pieces below which
// the data must be repaired to avoid loss
func (rs *RedundancyStrategy) RepairThreshold() int {
	return rs.repairThreshold
}

// OptimalThreshold is the number of available erasure pieces above which
// there is no need for the data to be repaired
func (rs *RedundancyStrategy) OptimalThreshold() int {
	return rs.optimalThreshold
}

type encodedReader struct {
	rs      RedundancyStrategy
	segment sync2.PipeReaderAt
	pieces  map[int](*encodedPiece)
}

// EncodeReader takes a Reader and a RedundancyStrategy and returns a slice of
// Readers.
//
// maxSize is the maximum number of bytes expected to be returned by the Reader.
func EncodeReader(ctx context.Context, r io.Reader, rs RedundancyStrategy, maxSize int64) ([]io.ReadCloser, error) {
	err := checkMaxSize(maxSize)
	if err != nil {
		return nil, err
	}

	er := &encodedReader{
		rs:     rs,
		pieces: make(map[int](*encodedPiece), rs.TotalCount()),
	}

	// TODO: make it configurable between file pipe and memory pipe
	teeReader, teeWriter, err := sync2.NewTeeFile("/tmp", rs.TotalCount())
	if err != nil {
		return nil, err
	}

	er.segment = teeReader
	readers := make([]io.ReadCloser, 0, rs.TotalCount())
	for i := 0; i < rs.TotalCount(); i++ {
		er.pieces[i] = &encodedPiece{
			er:        er,
			num:       i,
			stripeBuf: make([]byte, rs.StripeSize()),
			shareBuf:  make([]byte, rs.ErasureShareSize()),
		}
		readers = append(readers, er.pieces[i])
	}

	go er.fillBuffer(ctx, r, teeWriter)

	return readers, nil
}

func (er *encodedReader) fillBuffer(ctx context.Context, r io.Reader, w sync2.PipeWriter) {
	// TODO: interrupt copy if context is canceled
	_, err := io.Copy(w, r)
	err = w.CloseWithError(err)
	if err != nil {
		zap.S().Error(err)
	}
}

type encodedPiece struct {
	er            *encodedReader
	num           int
	currentStripe int64
	stripeBuf     []byte
	shareBuf      []byte
	available     int
	err           error
	closed        int32
}

func (ep *encodedPiece) Read(p []byte) (n int, err error) {
	if ep.err != nil {
		return 0, ep.err
	}

	if ep.available == 0 {
		// take the next stripe from the segment buffer
		off := ep.currentStripe * int64(ep.er.rs.StripeSize())
		_, err := ep.er.segment.ReadAt(ep.stripeBuf, off)
		if err != nil {
			return 0, err
		}

		// encode the num-th erasure share
		err = ep.er.rs.EncodeSingle(ep.stripeBuf, ep.shareBuf, ep.num)
		if err != nil {
			return 0, err
		}

		ep.currentStripe++
		ep.available = ep.er.rs.ErasureShareSize()
	}

	// we have some buffer remaining for this piece. write it to the output
	off := len(ep.shareBuf) - ep.available
	n = copy(p, ep.shareBuf[off:])
	ep.available -= n

	return n, nil
}

func (ep *encodedPiece) Close() error {
	if atomic.CompareAndSwapInt32(&ep.closed, 0, 1) {
		return ep.er.segment.Close()
	}
	return nil
}

// EncodedRanger will take an existing Ranger and provide a means to get
// multiple Ranged sub-Readers. EncodedRanger does not match the normal Ranger
// interface.
type EncodedRanger struct {
	rr      ranger.Ranger
	rs      RedundancyStrategy
	maxSize int64
}

// NewEncodedRanger from the given Ranger and RedundancyStrategy. See the
// comments for EncodeReader about the repair and optimal thresholds, and the
// max buffer memory.
func NewEncodedRanger(rr ranger.Ranger, rs RedundancyStrategy, maxSize int64) (*EncodedRanger, error) {
	if rr.Size()%int64(rs.StripeSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	if err := checkMaxSize(maxSize); err != nil {
		return nil, err
	}
	return &EncodedRanger{
		rs:      rs,
		rr:      rr,
		maxSize: maxSize,
	}, nil
}

// OutputSize is like Ranger.Size but returns the Size of the erasure encoded
// pieces that come out.
func (er *EncodedRanger) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.rs.StripeSize())
	return blocks * int64(er.rs.ErasureShareSize())
}

// Range is like Ranger.Range, but returns a slice of Readers
func (er *EncodedRanger) Range(ctx context.Context, offset, length int64) ([]io.ReadCloser, error) {
	// the offset and length given may not be block-aligned, so let's figure
	// out which blocks contain the request.
	firstBlock, blockCount := encryption.CalcEncompassingBlocks(
		offset, length, er.rs.ErasureShareSize())
	// okay, now let's encode the reader for the range containing the blocks
	r, err := er.rr.Range(ctx,
		firstBlock*int64(er.rs.StripeSize()),
		blockCount*int64(er.rs.StripeSize()))
	if err != nil {
		return nil, err
	}
	readers, err := EncodeReader(ctx, r, er.rs, er.maxSize)
	if err != nil {
		return nil, err
	}
	for i, r := range readers {
		// the offset might start a few bytes in, so we potentially have to
		// discard the beginning bytes
		_, err := io.CopyN(ioutil.Discard, r,
			offset-firstBlock*int64(er.rs.ErasureShareSize()))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		// the length might be shorter than a multiple of the block size, so
		// limit it
		readers[i] = readcloser.LimitReadCloser(r, length)
	}
	return readers, nil
}

func checkMaxSize(size int64) error {
	if size < 0 {
		return Error.New("negative max size")
	}
	return nil
}
