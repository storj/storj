// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import "time"

// Bucket contains information about a specific bucket
type Bucket struct {
	Name    string
	Created time.Time
}

// Object contains information about a specific object
type Object struct {
	Version  uint32
	Bucket   string
	Path     Path
	IsPrefix bool

	Metadata []byte

	ContentType string
	Created     time.Time
	Modified    time.Time
	Expires     time.Time

	Stream
}

// Stream is information about an object stream
type Stream struct {
	// Size is the total size of the stream in bytes
	Size int64
	// Checksum is the checksum of the segment checksums
	Checksum []byte

	// SegmentCount is the number of segments
	SegmentCount int64
	// FixedSegmentSize is the size of each segment,
	// when all segments have the same size. It is -1 otherwise.
	FixedSegmentSize int64

	// RedundancyScheme specifies redundancy strategy used for this stream
	RedundancyScheme
	// EncryptionScheme specifies encryption strategy used for this stream
	EncryptionScheme
}

// Segment is full segment information
type Segment struct {
	Index int64
	// Size is the size of the content in bytes
	Size int64
	// Checksum is the checksum of the content
	Checksum []byte
	// Local data
	Inline []byte
	// Remote data
	PieceID PieceID
	Pieces  []Piece
	// Encryption
	EncryptedKeyNonce Nonce
	EncryptedKey      EncryptedPrivateKey
}

// PieceID is an identificator for a piece
type PieceID []byte

// Piece is information where a piece is located
type Piece struct {
	Number   byte
	Location NodeID
}
