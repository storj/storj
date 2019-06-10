// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"

	"storj.io/storj/internal/readcloser"
	"storj.io/storj/pkg/ranger"
)

const (
	uint32Size = 4
)

// makePadding creates a slice of bytes of padding used to fill an encrytpion block.
// The last byte of the padding slice contains the count of the total padding bytes added.
func makePadding(paddingSize int) []byte {
	paddingBytes := bytes.Repeat([]byte{0}, paddingSize)
	binary.BigEndian.PutUint32(paddingBytes[paddingSize-uint32Size:], uint32(paddingSize))
	return paddingBytes
}

// calculatePaddingSize calculates how many bytes of padding are needed to fill
// an encryption block. Where dataLen is the number of bytes being encrypted,
// blocksize is the size of chunks that will be encrypted, and uint32Size is the amount
// of space needed to indicate how many total bytes of padding are added.
func calculatePaddingSize(dataLen int64, blockSize int) int {
	amount := dataLen + uint32Size
	r := amount % int64(blockSize)
	padding := uint32Size
	if r > 0 {
		padding += blockSize - int(r)
	}
	return padding
}

// Pad takes a Ranger and returns another Ranger that is a multiple of
// blockSize in length. The return value padding is a convenience to report how
// much padding was added.
func Pad(data ranger.Ranger, blockSize int) (
	rr ranger.Ranger, padding int) {
	paddingSize := calculatePaddingSize(data.Size(), blockSize)
	paddingBytes := makePadding(paddingSize)
	return ranger.Concat(data, ranger.ByteRanger(paddingBytes)), len(paddingBytes)
}

// Unpad takes a previously padded Ranger data source and returns an unpadded
// ranger, given the amount of padding. This is preferable to UnpadSlow if you
// can swing it.
func Unpad(data ranger.Ranger, padding int) (ranger.Ranger, error) {
	return ranger.Subrange(data, 0, data.Size()-int64(padding))
}

// UnpadSlow is like Unpad, but does not require the amount of padding.
// UnpadSlow will have to do extra work to make up for this missing information.
func UnpadSlow(ctx context.Context, data ranger.Ranger) (_ ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)
	r, err := data.Range(ctx, data.Size()-uint32Size, uint32Size)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	var p [uint32Size]byte
	_, err = io.ReadFull(r, p[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return Unpad(data, int(binary.BigEndian.Uint32(p[:])))
}

// PadReader is like Pad but works on a basic Reader instead of a Ranger.
func PadReader(data io.ReadCloser, blockSize int) io.ReadCloser {
	cr := newCountingReader(data)
	paddingSize := calculatePaddingSize(cr.N, blockSize)
	paddingBytes := makePadding(paddingSize)

	return readcloser.MultiReadCloser(cr,
		readcloser.LazyReadCloser(func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewReader(paddingBytes)), nil
		}))
}

type countingReader struct {
	R io.ReadCloser
	N int64
}

func newCountingReader(r io.ReadCloser) *countingReader {
	return &countingReader{R: r}
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.R.Read(p)
	cr.N += int64(n)
	return n, err
}

func (cr *countingReader) Close() error {
	return cr.R.Close()
}
