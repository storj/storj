// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"time"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
)

// DefaultRedundancy contains default redundancy scheme.
var DefaultRedundancy = storj.RedundancyScheme{
	Algorithm:      storj.ReedSolomon,
	ShareSize:      2048,
	RequiredShares: 1,
	RepairShares:   1,
	OptimalShares:  1,
	TotalShares:    1,
}

// DefaultEncryption contains default encryption parameters.
var DefaultEncryption = storj.EncryptionParameters{
	CipherSuite: storj.EncAESGCM,
	BlockSize:   29 * 256,
}

// DefaultRawSegment returns default raw segment.
func DefaultRawSegment(obj metabase.ObjectStream, segmentPosition metabase.SegmentPosition) metabase.RawSegment {
	return metabase.RawSegment{
		StreamID:    obj.StreamID,
		Position:    segmentPosition,
		RootPieceID: storj.PieceID{1},
		Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
		CreatedAt:   time.Now(),

		EncryptedKey:      []byte{3},
		EncryptedKeyNonce: []byte{4},
		EncryptedETag:     []byte{5},

		EncryptedSize: 1024,
		PlainSize:     512,
		PlainOffset:   0,
		Redundancy:    DefaultRedundancy,
	}
}

// DefaultRemoteRawSegment returns default remote raw segment.
func DefaultRemoteRawSegment(obj metabase.ObjectStream, segmentPosition metabase.SegmentPosition) metabase.RawSegment {
	pieces := metabase.Pieces{}
	for i := 0; i < 10; i++ {
		pieces = append(pieces, metabase.Piece{
			Number:      uint16(i),
			StorageNode: testrand.NodeID(),
		})
	}

	return metabase.RawSegment{
		StreamID:    obj.StreamID,
		Position:    segmentPosition,
		RootPieceID: testrand.PieceID(),

		Pieces:    pieces,
		CreatedAt: time.Now(),

		EncryptedKey:      testrand.Bytes(32),
		EncryptedKeyNonce: testrand.Bytes(32),

		EncryptedSize: 11 * memory.KiB.Int32(),
		PlainSize:     10 * memory.KiB.Int32(),
		PlainOffset:   0,
		Redundancy:    DefaultRedundancy,
	}
}
