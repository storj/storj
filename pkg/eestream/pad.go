// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"encoding/binary"
	"io"

	"storj.io/storj/pkg/ranger"
)

const (
	uint32Size = 4
)

func makePadding(dataLen int64, blockSize int) []byte {
	amount := dataLen + uint32Size
	r := amount % int64(blockSize)
	padding := uint32Size
	if r > 0 {
		padding += blockSize - int(r)
	}
	paddingBytes := bytes.Repeat([]byte{0}, padding)
	binary.BigEndian.PutUint32(paddingBytes[padding-uint32Size:], uint32(padding))
	return paddingBytes
}

// Pad takes a Ranger and returns another Ranger that is a multiple of
// blockSize in length. The return value padding is a convenience to report how
// much padding was added.
func Pad(data ranger.Ranger, blockSize int) (
	rr ranger.Ranger, padding int) {
	paddingBytes := makePadding(data.Size(), blockSize)
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
func UnpadSlow(data ranger.Ranger) (ranger.Ranger, error) {
	var p [uint32Size]byte
	_, err := io.ReadFull(data.Range(data.Size()-uint32Size, uint32Size), p[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return Unpad(data, int(binary.BigEndian.Uint32(p[:])))
}

// PadReader is like Pad but works on a basic Reader instead of a Ranger.
func PadReader(data io.Reader, blockSize int) io.Reader {
	cr := newCountingReader(data)
	return io.MultiReader(cr, ranger.LazyReader(func() io.Reader {
		return bytes.NewReader(makePadding(cr.N, blockSize))
	}))
}

type countingReader struct {
	R io.Reader
	N int64
}

func newCountingReader(r io.Reader) *countingReader {
	return &countingReader{R: r}
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
	n, err = cr.R.Read(p)
	cr.N += int64(n)
	return n, err
}
