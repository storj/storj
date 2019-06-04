package filters

import (
	"bytes"

	cuckoo "github.com/seiflotfy/cuckoofilter"
	"storj.io/storj/pkg/storj"
)

// PieceSlice is a slice of piece IDs
type PieceSlice []storj.PieceID

// Filter is an interface for filters
type Filter interface {
	Contains(pieceID []byte) bool
	Add(pieceID []byte)
}

// FilterConfig is a filter configuration
type FilterConfig struct {
	nbElements      int
	nbHashFunctions int
}

// PerfectSet is a perfect set
type PerfectSet struct {
	pieces      [][]byte
	maxSize     int
	currentSize int
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

// NewPerfectSet creates a set that can contain up to maxSize pieces
func NewPerfectSet(maxSize int) *PerfectSet {
	var p PerfectSet
	p.pieces = make([][]byte, maxSize)
	p.currentSize = 0
	return &p
}

// NewCuckooFilter returns a new cuckoo filter
func NewCuckooFilter(maxSize int) *CuckooFilter {
	var cf CuckooFilter
	cf.filter = cuckoo.NewFilter(uint(maxSize))
	return &cf
}

// Contains says if it's contained
func (p *PerfectSet) Contains(pieceID []byte) bool {
	return ArrayContains(pieceID, p.pieces)
}

// Add adds
func (p *PerfectSet) Add(pieceID []byte) {
	maxSize := len(p.pieces)
	if p.currentSize < maxSize {
		p.pieces[p.currentSize] = pieceID
		p.currentSize = p.currentSize + 1
	}
}

// ArrayContains returns true if the pieceIDs array of piece ids as byte contains pieceID
func ArrayContains(pieceID []byte, pieceIDs [][]byte) bool {
	for _, currentPieceID := range pieceIDs {
		if bytes.Equal(currentPieceID, pieceID) {
			return true
		}
	}
	return false
}
