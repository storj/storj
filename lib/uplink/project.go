// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
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
	tc            transport.Client
	metainfo      metainfo.Client
	project       *kvmetainfo.Project
	maxInlineSize memory.Size
}

// BucketConfig holds information about a bucket's configuration. This is
// filled in by the caller for use with CreateBucket(), or filled in by the
// library as Bucket.Config when a bucket is returned from OpenBucket().
type BucketConfig struct {
	// PathCipher indicates which cipher suite is to be used for path
	// encryption within the new Bucket. If not set, AES-GCM encryption
	// will be used.
	PathCipher storj.CipherSuite

	// EncryptionParameters specifies the default encryption parameters to
	// be used for data encryption of new Objects in this bucket.
	EncryptionParameters storj.EncryptionParameters

	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// RedundancyScheme defines the default Reed-Solomon and/or
		// Forward Error Correction encoding parameters to be used by
		// objects in this Bucket.
		RedundancyScheme storj.RedundancyScheme
		// SegmentSize is the default segment size to use for new
		// objects in this Bucket.
		SegmentSize memory.Size
	}
}

func (c *BucketConfig) setDefaults() {
	if c.PathCipher == storj.EncUnspecified {
		c.PathCipher = defaultCipher
	}
	if c.EncryptionParameters.CipherSuite == storj.EncUnspecified {
		c.EncryptionParameters.CipherSuite = defaultCipher
	}
	if c.EncryptionParameters.BlockSize == 0 {
		c.EncryptionParameters.BlockSize = (1 * memory.KiB).Int32()
	}
	if c.Volatile.RedundancyScheme.RequiredShares == 0 {
		c.Volatile.RedundancyScheme.RequiredShares = 29
	}
	if c.Volatile.RedundancyScheme.RepairShares == 0 {
		c.Volatile.RedundancyScheme.RepairShares = 35
	}
	if c.Volatile.RedundancyScheme.OptimalShares == 0 {
		c.Volatile.RedundancyScheme.OptimalShares = 80
	}
	if c.Volatile.RedundancyScheme.TotalShares == 0 {
		c.Volatile.RedundancyScheme.TotalShares = 95
	}
	if c.Volatile.RedundancyScheme.ShareSize == 0 {
		c.Volatile.RedundancyScheme.ShareSize = (1 * memory.KiB).Int32()
	}
	if c.Volatile.SegmentSize.Int() == 0 {
		c.Volatile.SegmentSize = 64 * memory.MiB
	}
}

// CreateBucket creates a new bucket if authorized.
func (p *Project) CreateBucket(ctx context.Context, name string, cfg *BucketConfig) (b storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	if cfg == nil {
		cfg = &BucketConfig{}
	}
	cfg.setDefaults()
	if cfg.Volatile.RedundancyScheme.ShareSize*int32(cfg.Volatile.RedundancyScheme.RequiredShares)%cfg.EncryptionParameters.BlockSize != 0 {
		return b, Error.New("EncryptionParameters.BlockSize must be a multiple of RS ShareSize * RS RequiredShares")
	}
	pathCipher := cfg.PathCipher.ToCipher()
	return p.project.CreateBucket(ctx, name, &storj.Bucket{PathCipher: pathCipher})
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
	// TODO(paul): fill in once info is plumbed
	cfg := &BucketConfig{}
	return b, cfg, nil
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
//
// maxMem is the default maximum amount of memory to be allocated for read
// buffers while performing decodes of objects in this Bucket. If set to a
// negative value, the system will use the smallest amount of memory it can. If
// set to zero, the library default amount of memory will be used.
func (p *Project) OpenBucket(ctx context.Context, bucket string, access *EncryptionAccess, maxMem memory.Size) (b *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketInfo, cfg, err := p.GetBucketInfo(ctx, bucket)
	if err != nil {
		return nil, err
	}

	if access == nil || access.Key == (storj.Key{}) {
		return nil, Error.New("No encryption key chosen")
	}
	pathCipher := cfg.PathCipher.ToCipher()
	if err != nil {
		return nil, err
	}
	encryptionScheme := cfg.EncryptionParameters.ToEncryptionScheme()

	if maxMem.Int() == 0 {
		maxMem = 4 * memory.MiB
	} else if maxMem.Int() < 0 {
		maxMem = 0
	}
	ec := ecclient.NewClient(p.tc, maxMem.Int())
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

	maxEncryptedSegmentSize, err := encryption.CalcEncryptedSize(cfg.Volatile.SegmentSize.Int64(),
		cfg.EncryptionParameters.ToEncryptionScheme())
	if err != nil {
		return nil, err
	}
	segments := segments.NewSegmentStore(p.metainfo, ec, rs, p.maxInlineSize.Int(), maxEncryptedSegmentSize)

	streams, err := streams.NewStreamStore(segments, cfg.Volatile.SegmentSize.Int64(), &access.Key, int(encryptionScheme.BlockSize), encryptionScheme.Cipher)
	if err != nil {
		return nil, err
	}

	buckets := buckets.NewStore(streams)

	return &Bucket{
		Bucket:     bucketInfo,
		Config:     *cfg,
		metainfo:   kvmetainfo.New(p.metainfo, buckets, streams, segments, &access.Key),
		streams:    streams,
		pathCipher: pathCipher,
	}, nil
}

// Close closes the Project.
func (p *Project) Close() error {
	return nil
}
