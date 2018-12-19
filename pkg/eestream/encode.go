// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"context"
	"io"
	"io/ioutil"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/utils"
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

// SyncBuffer is a synchronized variable-sized buffer used to buffer the
// generated pieces by erasure coding. Read will block until new data is added
// or the buffer is closed. If the buffer is closed, Read will continue
// reading as long as there is data in the buffer.
type SyncBuffer interface {
	io.Reader
	io.Writer
	io.Closer
	CloseWithError(err error) error
}

type encodedReader struct {
	r     io.Reader
	rs    RedundancyStrategy
	inbuf []byte
	eps   map[int]SyncBuffer
}

// EncodeReader takes a Reader and a RedundancyStrategy and returns a slice of
// Readers.
//
// expectedReaderSize is the size of the expected data in the r reader. It will
// be used to allocate the necessary internal buffers and avoid unnecessary
// buffer resizing. Set to 0 if not known.
func EncodeReader(ctx context.Context, r io.Reader, rs RedundancyStrategy, expectedReaderSize int64) ([]io.Reader, error) {
	if err := checkExpectedReaderSize(expectedReaderSize); err != nil {
		return nil, err
	}

	pieceSize := expectedReaderSize / int64(rs.RequiredCount())

	er := &encodedReader{
		r:     r,
		rs:    rs,
		inbuf: make([]byte, rs.StripeSize()),
		eps:   make(map[int]SyncBuffer, rs.TotalCount()),
	}

	readers := make([]io.Reader, 0, rs.TotalCount())
	for i := 0; i < rs.TotalCount(); i++ {
		er.eps[i] = sync2.NewMemoryBuffer(make([]byte, 0, pieceSize))
		readers = append(readers, er.eps[i])
	}

	go er.fillBuffer(ctx)

	return readers, nil
}

func (er *encodedReader) fillBuffer(ctx context.Context) {
	var err error

	for {
		if ctx.Err() == context.Canceled {
			err = context.Canceled
			break
		}

		_, err = io.ReadFull(er.r, er.inbuf)
		if err != nil {
			break
		}

		var writeErr error
		err = er.rs.Encode(er.inbuf, func(num int, data []byte) {
			_, writeErr = er.eps[num].Write(data)
		})

		err = utils.CombineErrors(err, writeErr)
		if err != nil {
			break
		}
	}

	if err == io.EOF {
		err = nil
	}

	var errGroup utils.ErrorGroup
	for _, buf := range er.eps {
		errGroup.Add(buf.CloseWithError(err))
	}

	err = errGroup.Finish()
	if err != nil {
		zap.S().Errorf("Error closing buffers: %v", err)
	}
}

// EncodedRanger will take an existing Ranger and provide a means to get
// multiple Ranged sub-Readers. EncodedRanger does not match the normal Ranger
// interface.
type EncodedRanger struct {
	rr                 ranger.Ranger
	rs                 RedundancyStrategy
	expectedReaderSize int64
}

// NewEncodedRanger from the given Ranger and RedundancyStrategy. See the
// comments for EncodeReader about the repair and optimal thresholds, and the
// max buffer memory.
func NewEncodedRanger(rr ranger.Ranger, rs RedundancyStrategy, expectedReaderSize int64) (*EncodedRanger, error) {
	if rr.Size()%int64(rs.StripeSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	if err := checkExpectedReaderSize(expectedReaderSize); err != nil {
		return nil, err
	}
	return &EncodedRanger{
		rs:                 rs,
		rr:                 rr,
		expectedReaderSize: expectedReaderSize,
	}, nil
}

// OutputSize is like Ranger.Size but returns the Size of the erasure encoded
// pieces that come out.
func (er *EncodedRanger) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.rs.StripeSize())
	return blocks * int64(er.rs.ErasureShareSize())
}

// Range is like Ranger.Range, but returns a slice of Readers
func (er *EncodedRanger) Range(ctx context.Context, offset, length int64) ([]io.Reader, error) {
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
	readers, err := EncodeReader(ctx, r, er.rs, er.expectedReaderSize)
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
		readers[i] = io.LimitReader(r, length)
	}
	return readers, nil
}

func checkExpectedReaderSize(size int64) error {
	if size < 0 {
		return Error.New("negative expected reader size")
	}
	return nil
}
