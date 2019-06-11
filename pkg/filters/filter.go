// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filters

import (
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

// Filter is an interface for filters
type Filter interface {
	Contains(pieceID []byte) bool
	Add(pieceID []byte)
	Encode() []byte
}

// FilterConfig is a filter configuration
type FilterConfig struct {
	nbElements      int
	nbHashFunctions int
}

// CuckooFilter is a cuckoo filter
type CuckooFilter struct {
	filter *cuckoo.Filter
}

// Add adds
func (cf *CuckooFilter) Add(pieceID []byte) {
	cf.filter.Insert(pieceID)
}

// Contains returns true if it is contained
func (cf *CuckooFilter) Contains(pieceID []byte) bool {
	return cf.filter.Lookup(pieceID)

}

// Encode returns an array of bytes representing the filter
func (cf *CuckooFilter) Encode() []byte {
	return cf.filter.Encode()
}

// NewCuckooFilter returns a new cuckoo filter
func NewCuckooFilter(maxSize int) *CuckooFilter {
	var cf CuckooFilter

	cf.filter = cuckoo.NewFilter(uint(maxSize))
	return &cf
}
