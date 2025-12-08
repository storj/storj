// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"time"

	"github.com/stretchr/testify/require"

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

// RandEncryptedUserData returns full randomized encrypted user data.
func RandEncryptedUserData() metabase.EncryptedUserData {
	return metabase.EncryptedUserData{
		EncryptedMetadata:             testrand.Bytes(32),
		EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
		EncryptedMetadataEncryptedKey: testrand.Bytes(48),
		EncryptedETag:                 testrand.Bytes(32),
	}
}

// RandEncryptedUserDataWithoutETag returns full randomized encrypted user data.
func RandEncryptedUserDataWithoutETag() metabase.EncryptedUserData {
	return metabase.EncryptedUserData{
		EncryptedMetadata:             testrand.Bytes(32),
		EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
		EncryptedMetadataEncryptedKey: testrand.Bytes(48),
	}
}

// EncryptedUserDataScenario is data definition for invalid user data.
type EncryptedUserDataScenario struct {
	EncryptedUserData metabase.EncryptedUserData
	ErrText           string
}

// InvalidEncryptedUserDataScenarios returns user data examples that are invalid.
func InvalidEncryptedUserDataScenarios() []EncryptedUserDataScenario {
	return []EncryptedUserDataScenario{
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadata: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set when EncryptedMetadata or EncryptedETag are set",
		},
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedETag: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set when EncryptedMetadata or EncryptedETag are set",
		},
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadata: []byte{1},
				EncryptedETag:     []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set when EncryptedMetadata or EncryptedETag are set",
		},
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadata:      []byte{1},
				EncryptedETag:          []byte{1},
				EncryptedMetadataNonce: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must always be set together",
		},
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadata:             []byte{1},
				EncryptedETag:                 []byte{1},
				EncryptedMetadataEncryptedKey: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must always be set together",
		},

		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadataNonce: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must always be set together",
		},
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadataEncryptedKey: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must always be set together",
		},
		{
			EncryptedUserData: metabase.EncryptedUserData{
				EncryptedMetadataNonce:        []byte{1},
				EncryptedMetadataEncryptedKey: []byte{1},
			},
			ErrText: "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be empty when EncryptedMetadata or EncryptedETag are empty",
		},
	}
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

// DefaultRawInlineSegment returns default inline raw segment.
func DefaultRawInlineSegment(obj metabase.ObjectStream, segmentPosition metabase.SegmentPosition) metabase.RawSegment {
	return metabase.RawSegment{
		StreamID:  obj.StreamID,
		Position:  segmentPosition,
		CreatedAt: time.Now(),

		EncryptedKey:      []byte{3},
		EncryptedKeyNonce: []byte{4},
		EncryptedETag:     []byte{5},

		EncryptedSize: 3,
		PlainSize:     2,
		PlainOffset:   0,
		InlineData:    []byte{1, 2, 3},
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

// EqualRetention asserts that two Retention values are equal.
func EqualRetention(t require.TestingT, expected, actual metabase.Retention) {
	require.Equal(t, expected.Mode, actual.Mode)
	// use Microsecond delta to match Postgres precision
	require.WithinDuration(t, expected.RetainUntil, actual.RetainUntil, time.Microsecond)
}
