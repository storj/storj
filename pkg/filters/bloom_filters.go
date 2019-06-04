package filters

import (
	"encoding/binary"

	steakknife "github.com/steakknife/bloomfilter"
	willf "github.com/willf/bloom"
	zeebo "github.com/zeebo/sbloom"

	"hash/fnv"
)

// ZeeboBloomFilter represents zeebo's bloom filter implementation
type ZeeboBloomFilter struct {
	filter *zeebo.Filter
}

// WillfBloomFilter is willf's bloom filter implementation
type WillfBloomFilter struct {
	filter *willf.BloomFilter
}

// SteakknifeBloomFilter is a bloom filter from steakknife
type SteakknifeBloomFilter struct {
	filter *steakknife.Filter
}

// NewZeeboBloomFilter returns a zeebo bloom filter
func NewZeeboBloomFilter() *ZeeboBloomFilter {
	var zbf ZeeboBloomFilter
	zbf.filter = zeebo.NewFilter(fnv.New64(), 10)
	return &zbf
}

// Add adds a pieceID
func (zbf *ZeeboBloomFilter) Add(pieceID []byte) {
	zbf.filter.Add(pieceID)
}

// Contains returns true if the pieceID may be in the set
func (zbf *ZeeboBloomFilter) Contains(pieceID []byte) bool {
	return zbf.filter.Lookup(pieceID)
}

// NewWillfBloomFilter returns a bloom filter of size size
func NewWillfBloomFilter(maxElements uint, p float64) *WillfBloomFilter {
	var wbf WillfBloomFilter
	m := steakknife.OptimalM(uint64(maxElements), p)
	wbf.filter = willf.New(uint(m), uint(steakknife.OptimalK(uint64(m), uint64(maxElements))))
	return &wbf
}

// Add adds pieceID to the set
func (wbf *WillfBloomFilter) Add(pieceID []byte) {
	wbf.filter.Add(pieceID)
}

// Contains return true if pieceID may be in the set
func (wbf *WillfBloomFilter) Contains(pieceID []byte) bool {
	return wbf.filter.Test(pieceID)
}

// NewSteakknifeBloomFilter creates a new SteakknifeBloomFilter
func NewSteakknifeBloomFilter(maxElements uint64, p float64) *SteakknifeBloomFilter {
	var sbf SteakknifeBloomFilter
	var err error
	sbf.filter, err = steakknife.NewOptimal(maxElements, p)
	if err != nil {

	}
	return &sbf
}

type hashableByteArray []byte

func (h hashableByteArray) Write([]byte) (int, error) {
	panic("Unimplemented")
}

func (h hashableByteArray) Sum([]byte) []byte {
	panic("Unimplemented")
}

func (h hashableByteArray) Reset() {
	panic("Unimplemented")
}

func (h hashableByteArray) BlockSize() int {
	panic("Unimplemented")
}

func (h hashableByteArray) Size() int {
	panic("Unimplemented")
}

func (h hashableByteArray) Sum64() uint64 {
	return binary.BigEndian.Uint64(h)
}

// Add adds pieceID to the set
func (sbf *SteakknifeBloomFilter) Add(pieceID []byte) {
	sbf.filter.Add(hashableByteArray(pieceID))
}

// Contains return true if pieceID may be in the set
func (sbf *SteakknifeBloomFilter) Contains(pieceID []byte) bool {
	return sbf.filter.Contains(hashableByteArray(pieceID))
}
