// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !race

package metainfo

import (
	"storj.io/common/pb"
	"storj.io/storj/satellite/metainfo/metabase"
)

// sanityCheckPointer implements sanity checking test data,
// we don't need this in production code.
func sanityCheckPointer(key metabase.SegmentKey, pointer *pb.Pointer) (err error) {
	return nil
}
