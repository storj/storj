// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"github.com/vivint/infectious"
)

type rsScheme struct {
	fc        *infectious.FEC
	blockSize int
}

// NewRSScheme returns a Reed-Solomon-based ErasureScheme.
func NewRSScheme(fc *infectious.FEC, blockSize int) ErasureScheme {
	return &rsScheme{fc: fc, blockSize: blockSize}
}

func (s *rsScheme) Encode(input []byte, output func(num int, data []byte)) (
	err error) {
	return s.fc.Encode(input, func(s infectious.Share) {
		output(s.Number, s.Data)
	})
}

func (s *rsScheme) Decode(out []byte, in map[int][]byte) ([]byte, error) {
	shares := make([]infectious.Share, 0, len(in))
	for num, data := range in {
		shares = append(shares, infectious.Share{Number: num, Data: data})
	}
	return s.fc.Decode(out, shares)
}

func (s *rsScheme) EncodedBlockSize() int {
	return s.blockSize
}

func (s *rsScheme) DecodedBlockSize() int {
	return s.blockSize * s.fc.Required()
}

func (s *rsScheme) TotalCount() int {
	return s.fc.Total()
}

func (s *rsScheme) RequiredCount() int {
	return s.fc.Required()
}
