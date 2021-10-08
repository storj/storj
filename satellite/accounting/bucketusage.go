// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"time"

	"storj.io/common/uuid"
)

// BucketStorageTally holds data about a bucket tally.
type BucketStorageTally struct {
	BucketName    string
	ProjectID     uuid.UUID
	IntervalStart time.Time

	ObjectCount int64

	TotalSegmentCount int64
	TotalBytes        int64

	MetadataSize int64
}

// Bytes returns total bytes.
func (s *BucketStorageTally) Bytes() int64 {
	return s.TotalBytes
}
