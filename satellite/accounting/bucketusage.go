// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// BucketStorageTally holds data about a bucket tally
type BucketStorageTally struct {
	BucketName    string
	ProjectID     uuid.UUID
	IntervalStart time.Time

	ObjectCount int64

	InlineSegmentCount int64
	RemoteSegmentCount int64

	InlineBytes  int64
	RemoteBytes  int64
	MetadataSize int64
}
