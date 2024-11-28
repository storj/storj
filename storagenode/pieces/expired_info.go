// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"fmt"
	"unsafe"

	"storj.io/common/storj"
)

type expirationRecord struct {
	PieceID   storj.PieceID
	PieceSize int64
}

const expirationRecordSize = unsafe.Sizeof(expirationRecord{})

// ExpiredInfoRecords contains piece expiration information. The piece
// information is stored in a contiguous slab of memory in hopes of avoiding
// allocations and fragmented memory (these lists can get quite big).
type ExpiredInfoRecords struct {
	// SatelliteID is the satellite to which all of these piece expiration
	// records belong.
	SatelliteID storj.NodeID

	// InPieceInfo indicates whether these records were taken from the
	// v0 pieceinfo database or not.
	InPieceInfo bool

	// records contains the records. Access elements using Index(), Append(),
	// etc.
	records []expirationRecord
}

// NewExpiredInfoRecords creates a new ExpiredInfoRecords.
func NewExpiredInfoRecords(satelliteID storj.NodeID, isInPieceInfoDB bool, predictedLen int) *ExpiredInfoRecords {
	return &ExpiredInfoRecords{
		SatelliteID: satelliteID,
		InPieceInfo: isInPieceInfoDB,
		records:     make([]expirationRecord, 0, predictedLen),
	}
}

// Len returns the number of piece expiration records stored.
func (e *ExpiredInfoRecords) Len() int {
	return len(e.records)
}

// Index returns a single piece expiration record, as given by the 0-based index
// argument. i must be less than Len().
func (e *ExpiredInfoRecords) Index(i int) ExpiredInfo {
	return ExpiredInfo{
		SatelliteID: e.SatelliteID,
		PieceID:     e.records[i].PieceID,
		PieceSize:   e.records[i].PieceSize,
		InPieceInfo: e.InPieceInfo,
	}
}

// PieceIDAtIndex works like Index, but only returns the PieceID and PieceSize,
// for cases when only they are needed. This avoids copying the satelliteID
// potentially millions of times.
func (e *ExpiredInfoRecords) PieceIDAtIndex(i int) (pieceID storj.PieceID, size int64) {
	return e.records[i].PieceID, e.records[i].PieceSize
}

// Append adds a piece expiration record to the list. If not enough memory was
// allocated initially, this will cause a reallocation.
func (e *ExpiredInfoRecords) Append(pieceID storj.PieceID, pieceSize int64) {
	e.records = append(e.records, expirationRecord{
		PieceID:   pieceID,
		PieceSize: pieceSize,
	})
}

func (e *ExpiredInfoRecords) String() string {
	return fmt.Sprintf("{SatelliteID: %s, InPieceInfo: %v, records: %v}", e.SatelliteID, e.InPieceInfo, e.records)
}
