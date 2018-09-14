// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"encoding/binary"
	"hash/crc32"

	"storj.io/storj/pkg/ranger"
)

const (
	crcBlockSize = 64 // this could literally be whatever
	uint64Size   = 8
)

// crcAdder is a Transformer that is going to add a block number and a crc to
// the end of each block
type crcAdder struct {
	Table *crc32.Table
}

func newCRCAdder(t *crc32.Table) *crcAdder {
	return &crcAdder{Table: t}
}

func (c *crcAdder) InBlockSize() int { return crcBlockSize }
func (c *crcAdder) OutBlockSize() int {
	return crcBlockSize + uint32Size + uint64Size
}

func (c *crcAdder) Transform(out, in []byte, blockOffset int64) (
	[]byte, error) {
	// we're just going to take the input data, then add the block number,
	// big-endian encoded, then add the big-endian crc of the input + block
	// number.
	out = append(out, in...)
	var buf [uint64Size]byte
	binary.BigEndian.PutUint64(buf[:], uint64(blockOffset))
	out = append(out, buf[:]...)
	binary.BigEndian.PutUint32(buf[:uint32Size], crc32.Checksum(out, c.Table))
	out = append(out, buf[:uint32Size]...)
	return out, nil
}

// crcChecker is a Transformer that validates a given CRC and compares the
// block number, then removes them from the input, returning the original
// unchecked input.
type crcChecker struct {
	Table *crc32.Table
}

func newCRCChecker(t *crc32.Table) *crcChecker {
	return &crcChecker{Table: t}
}

func (c *crcChecker) InBlockSize() int {
	return crcBlockSize + uint32Size + uint64Size
}

func (c *crcChecker) OutBlockSize() int { return crcBlockSize }

func (c *crcChecker) Transform(out, in []byte, blockOffset int64) (
	[]byte, error) {
	bs := c.OutBlockSize()
	// first check the crc
	if binary.BigEndian.Uint32(in[bs+uint64Size:bs+uint64Size+uint32Size]) !=
		crc32.Checksum(in[:bs+uint64Size], c.Table) {
		return nil, Error.New("crc check mismatch")
	}
	// then check the block offset
	if binary.BigEndian.Uint64(in[bs:bs+uint64Size]) != uint64(blockOffset) {
		return nil, Error.New("block offset mismatch")
	}
	return append(out, in[:bs]...), nil
}

// addCRC is a Ranger constructor, given a specific crc table and an existing
// un-crced Ranger
func addCRC(data ranger.Ranger, tab *crc32.Table) (ranger.Ranger, error) {
	return Transform(data, newCRCAdder(tab))
}

// checkCRC is a Ranger constructor, given a specific crc table and an existing
// crced Ranger
func checkCRC(data ranger.Ranger, tab *crc32.Table) (ranger.Ranger, error) {
	return Transform(data, newCRCChecker(tab))
}
