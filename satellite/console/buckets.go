// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Buckets is interface for working with bucket to project relations
type Buckets interface {
	// ListBuckets returns bucket list of a given project
	ListBuckets(ctx context.Context, projectID uuid.UUID) ([]Bucket, error)
	// GetBucket retrieves bucket info of bucket with given name
	GetBucket(ctx context.Context, name string) (*Bucket, error)
	// AttachBucket attaches a bucket to a project
	AttachBucket(ctx context.Context, name string, projectID uuid.UUID) (*Bucket, error)
	// DeattachBucket deletes bucket info for a bucket by name
	DeattachBucket(ctx context.Context, name string) error
}

// Bucket represents bucket to project relationship
type Bucket struct {
	Name string

	ProjectID uuid.UUID

	CreatedAt time.Time
}
