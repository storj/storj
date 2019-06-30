// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bucketsDB

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

)

// DB is the interface for the database to interact with buckets
type DB interface {
	// Create creates a new bucket
	Create(context.Context, Bucket) error
	// Get returns an existing bucket
	Get(context.Context, bucketID uuid.UUID) (Bucket, error)
	// Delete deletes a bucket in the database
	Delete(context.Context, bucketID uuid.UUID) error
	// List returns all buckets for a project
	List(context.Context, projectID uuid.UUID) ([]Bucket, error)
}

type Bucket struct {}
