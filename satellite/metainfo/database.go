// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
)

// Bucket defines internal implementation of buckets
type Bucket struct {
	ID uuid.UUID

	ProjectID  uuid.UUID
	Name       string
	PathCipher storj.Cipher

	AttributionID uuid.UUID // []byte?
	CreatedAt     time.Time

	// do we need "Default" prefix here?
	DefaultSegmentSize int64
	DefaultRedundancy  storj.RedundancyScheme
	DefaultEncryption  storj.EncryptionParameters
}

type Buckets interface {
	Create(ctx context.Context, bucket *Bucket) error
	Get(ctx context.Context, projectID uuid.UUID, name string) (*Bucket, error)
	Delete(ctx context.Context, projectID uuid.UUID, name string) error
	List(ctx context.Context, projectID uuid.UUID, opts storj.BucketListOptions) (storj.BucketList, error)
}
