// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/segments"
	"storj.io/storj/uplink/storage/streams"
)

var (
	// Error is the errs class of SetupProject
	Error = errs.Class("SetupProject error")
)

// SetupProject creates a project with temporary values until we can figure out how to bypass encryption related setup
func SetupProject(m *metainfo.Client) (*Project, error) {
	whoCares := 1 // TODO: find a better way to do this
	fc, err := infectious.NewFEC(whoCares, whoCares)
	if err != nil {
		return nil, Error.New("failed to create erasure coding client: %v", err)
	}
	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, whoCares), whoCares, whoCares)
	if err != nil {
		return nil, Error.New("failed to create redundancy strategy: %v", err)
	}
	maxBucketMetaSize := 10 * memory.MiB
	segment := segments.NewSegmentStore(m, nil, rs)

	// volatile warning: we're setting an encryption key of all zeros for bucket
	// metadata, when really the bucket metadata should be stored in a different
	// system altogether.
	// TODO: https://storjlabs.atlassian.net/browse/V3-1967
	encStore := encryption.NewStore()
	encStore.SetDefaultKey(new(storj.Key))
	strms, err := streams.NewStreamStore(m, segment, maxBucketMetaSize.Int64(), encStore, memory.KiB.Int(), storj.EncAESGCM, maxBucketMetaSize.Int(), maxBucketMetaSize.Int64())
	if err != nil {
		return nil, Error.New("failed to create streams: %v", err)
	}

	return NewProject(strms, memory.KiB.Int32(), rs, 64*memory.MiB.Int64(), *m), nil
}
