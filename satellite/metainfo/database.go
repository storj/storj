// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
)

// this is mostly copy paste from storj.Bucket
type Bucket struct {
	ID        uuid.UUID
	ProjectID uuid.UUID

	Name    string
	Created time.Time

	AttributionID uuid.UUID // []byte?

	PathCipher storj.Cipher

	// do we need "Default" prefix here?
	DefaultSegmentSize int64
	DefaultRedundancy  storj.RedundancyScheme
	DefaultEncryption  storj.EncryptionParameters
}

type Checksum uint64

type Object struct {
	BucketID      uuid.UUID
	EncryptedPath storj.Path
	StreamID      uuid.UUID

	Status  ObjectStatus
	Version uint32

	Created time.Time
	Expires time.Time

	EncryptedMetadata []byte

	DataChecksum     Checksum
	TotalSize        int64
	FixedSegmentSize int64
	SegmentCount     int64

	Encryption storj.EncryptionParameters
	Redundancy storj.RedundancyScheme
}

type ObjectStatus byte

const (
	Partial ObjectStatus = iota
	Committing
	Committed
	Deleting
)

type Segment struct {
	StreamID     uuid.UUID
	SegmentIndex uint64

	RootPieceID storj.PieceID

	EncryptedKeyNonce storj.Nonce
	EncryptedKey      storj.EncryptedPrivateKey

	EncryptedDataChecksum Checksum
	EncryptedDataSize     int64
	EncryptedInlineData   []byte
	Nodes                 []storj.NodeID
}

type Buckets interface {
	Create(ctx context.Context, bucket *Bucket) error
	Get(ctx context.Context, projectID uuid.UUID, name string) (*Bucket, error)
	Delete(ctx context.Context, projectID uuid.UUID, name string) error
	List(ctx context.Context, projectID uuid.UUID, opts storj.BucketListOptions) (storj.BucketList, error)
}

type Objects interface {
	Get(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) (*Object, error)
	List(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) ([]*Object, error)
	Delete(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) error

	GetPartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) (*Object, error)
	ListPartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) ([]*Object, error)
	DeletePartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) error

	Create(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32, object *Object) error
	Commit(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) (*Object, error)
}

type Segments interface {
	Create(ctx context.Context, segment *Segment) error
	Commit(ctx context.Context, segment *Segment) error
	Get(ctx context.Context, streamID uuid.UUID, segmentIndex int64) ([]*Segment, error)
	List(ctx context.Context, streamID uuid.UUID, segmentIndex int64, limit int) ([]*Segment, error)
}
