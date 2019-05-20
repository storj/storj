// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/storage/buckets"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/metainfo"
)

// Project represents a specific project access session.
type Project struct {
	uplinkCfg     *Config
	tc            transport.Client
	metainfo      metainfo.Client
	project       *kvmetainfo.Project
	maxInlineSize memory.Size
}

// BucketConfig holds information about a bucket's configuration. This is
// filled in by the caller for use with CreateBucket(), or filled in by the
// library as Bucket.Config when a bucket is returned from OpenBucket().
type BucketConfig struct {
	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// RedundancyScheme defines the default Reed-Solomon and/or
		// Forward Error Correction encoding parameters to be used by
		// objects in this Bucket.
		RedundancyScheme storj.RedundancyScheme
		// SegmentsSize is the default segment size to use for new
		// objects in this Bucket.
		SegmentsSize memory.Size
	}
}

func (cfg *BucketConfig) clone() *BucketConfig {
	clone := *cfg
	return &clone
}

func (cfg *BucketConfig) setDefaults() {
	if cfg.Volatile.RedundancyScheme.RequiredShares == 0 {
		cfg.Volatile.RedundancyScheme.RequiredShares = 29
	}
	if cfg.Volatile.RedundancyScheme.RepairShares == 0 {
		cfg.Volatile.RedundancyScheme.RepairShares = 35
	}
	if cfg.Volatile.RedundancyScheme.OptimalShares == 0 {
		cfg.Volatile.RedundancyScheme.OptimalShares = 80
	}
	if cfg.Volatile.RedundancyScheme.TotalShares == 0 {
		cfg.Volatile.RedundancyScheme.TotalShares = 95
	}
	if cfg.Volatile.RedundancyScheme.ShareSize == 0 {
		cfg.Volatile.RedundancyScheme.ShareSize = (1 * memory.KiB).Int32()
	}
	if cfg.Volatile.SegmentsSize.Int() == 0 {
		cfg.Volatile.SegmentsSize = 64 * memory.MiB
	}
}

// CreateBucket creates a new bucket if authorized.
func (p *Project) CreateBucket(ctx context.Context, name string, cfg *BucketConfig) (b storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	if cfg == nil {
		cfg = &BucketConfig{}
	}
	cfg = cfg.clone()
	cfg.setDefaults()

	b = storj.Bucket{
		RedundancyScheme:     cfg.Volatile.RedundancyScheme,
		SegmentsSize:         cfg.Volatile.SegmentsSize.Int64(),
	}
	return p.project.CreateBucket(ctx, name, &b)
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
func (p *Project) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return p.project.DeleteBucket(ctx, bucket)
}

// BucketListOptions controls options to the ListBuckets() call.
type BucketListOptions = storj.BucketListOptions

// ListBuckets will list authorized buckets.
func (p *Project) ListBuckets(ctx context.Context, opts *BucketListOptions) (bl storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts == nil {
		opts = &BucketListOptions{}
	}
	return p.project.ListBuckets(ctx, *opts)
}

// GetBucketInfo returns info about the requested bucket if authorized.
func (p *Project) GetBucketInfo(ctx context.Context, bucket string) (b storj.Bucket, bi *BucketConfig, err error) {
	defer mon.Task()(&ctx)(&err)
	b, err = p.project.GetBucket(ctx, bucket)
	if err != nil {
		return b, nil, err
	}
	cfg := &BucketConfig{}
	cfg.Volatile.RedundancyScheme = b.RedundancyScheme
	cfg.Volatile.SegmentsSize = memory.Size(b.SegmentsSize)
	return b, cfg, nil
}

// OpenBucket returns a Bucket handle
func (p *Project) OpenBucket(ctx context.Context, bucketName string, access *EncryptionAccess) (b *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketInfo, cfg, err := p.GetBucketInfo(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	ec := ecclient.NewClient(p.tc, p.uplinkCfg.Volatile.MaxMemory.Int())
	fc, err := infectious.NewFEC(int(cfg.Volatile.RedundancyScheme.RequiredShares), int(cfg.Volatile.RedundancyScheme.TotalShares))
	if err != nil {
		return nil, err
	}
	rs, err := eestream.NewRedundancyStrategy(
		eestream.NewRSScheme(fc, int(cfg.Volatile.RedundancyScheme.ShareSize)),
		int(cfg.Volatile.RedundancyScheme.RepairShares),
		int(cfg.Volatile.RedundancyScheme.OptimalShares))
	if err != nil {
		return nil, err
	}

	segmentStore := segments.NewSegmentStore(p.metainfo, ec, rs, p.maxInlineSize.Int())

	streamStore, err := streams.NewStreamStore(segmentStore, cfg.Volatile.SegmentsSize.Int64(), &access.Key)
	if err != nil {
		return nil, err
	}

	bucketStore := buckets.NewStore(streamStore)

	return &Bucket{
		BucketConfig: *cfg,
		Name:         bucketInfo.Name,
		Created:      bucketInfo.Created,
		bucket:       bucketInfo,
		metainfo:     kvmetainfo.New(p.metainfo, bucketStore, streamStore, segmentStore, &access.Key, rs, cfg.Volatile.SegmentsSize.Int64()),
		streams:      streamStore,
	}, nil
}

// Close closes the Project.
func (p *Project) Close() error {
	return nil
}
