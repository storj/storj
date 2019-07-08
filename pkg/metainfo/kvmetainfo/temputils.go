// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/metainfo"
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
	segment := segments.NewSegmentStore(m, nil, rs, maxBucketMetaSize.Int(), maxBucketMetaSize.Int64())

	// volatile warning: we're setting an encryption key of all zeros for bucket
	// metadata, when really the bucket metadata should be stored in a different
	// system altogether.
	// TODO: https://storjlabs.atlassian.net/browse/V3-1967
	encStore := encryption.NewStore()
	encStore.SetDefaultKey(new(storj.Key))
	strms, err := streams.NewStreamStore(segment, maxBucketMetaSize.Int64(), encStore, memory.KiB.Int(), storj.EncAESGCM, maxBucketMetaSize.Int())
	if err != nil {
		return nil, Error.New("failed to create streams: %v", err)
	}

	return NewProject(strms, memory.KiB.Int32(), rs, 64*memory.MiB.Int64()), nil
}
