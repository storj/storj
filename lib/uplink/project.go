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
	tc       transport.Client
	metainfo metainfo.Client
	project  *kvmetainfo.Project
}

type CreateBucketOptions struct {
	PathCipher Cipher
}

func (o *CreateBucketOptions) setDefaults() {
	if o.PathCipher == UnsetCipher {
		o.PathCipher = defaultCipher
	}
}

// CreateBucket creates a new bucket if authorized
func (p *Project) CreateBucket(ctx context.Context, bucket string, opts CreateBucketOptions) (b storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	opts.setDefaults()
	pathCipher, err := opts.PathCipher.convert()
	if err != nil {
		return storj.Bucket{}, err
	}
	return p.project.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: pathCipher})
}

// DeleteBucket deletes a bucket if authorized
func (p *Project) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return p.project.DeleteBucket(ctx, bucket)
}

// ListBuckets will list authorized buckets
func (p *Project) ListBuckets(ctx context.Context, opts storj.BucketListOptions) (bl storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	return p.project.ListBuckets(ctx, opts)
}

// GetBucketInfo returns info about the requested bucket if authorized
func (p *Project) GetBucketInfo(ctx context.Context, bucket string) (b storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return p.project.GetBucket(ctx, bucket)
}

// BucketConfig represents configuration options for a specific Bucket
type BucketConfig struct {
	EncryptionAccess EncryptionAccess

	// These config values are likely to change semantics or go away
	// entirely between releases. Be careful when using them!
	Volatile struct {
		DefaultRS struct {
			MinThreshold     int
			RepairThreshold  int
			SuccessThreshold int
			MaxThreshold     int
		}

		MaxBufferMem     memory.Size
		ErasureShareSize memory.Size
		SegmentSize      memory.Size
		MaxInlineSize    memory.Size

		DataCipher          Cipher
		PathCipher          Cipher
		EncryptionBlockSize memory.Size
	}
}

func (c *BucketConfig) setDefaults() {
	if c.Volatile.PathCipher == UnsetCipher {
		c.Volatile.PathCipher = defaultCipher
	}
	if c.Volatile.DataCipher == UnsetCipher {
		c.Volatile.DataCipher = defaultCipher
	}
	if c.Volatile.EncryptionBlockSize.Int() == 0 {
		c.Volatile.EncryptionBlockSize.Set("1KiB")
	}
	if c.Volatile.DefaultRS.MinThreshold == 0 {
		c.Volatile.DefaultRS.MinThreshold = 29
	}
	if c.Volatile.DefaultRS.RepairThreshold == 0 {
		c.Volatile.DefaultRS.RepairThreshold = 35
	}
	if c.Volatile.DefaultRS.SuccessThreshold == 0 {
		c.Volatile.DefaultRS.SuccessThreshold = 80
	}
	if c.Volatile.DefaultRS.MaxThreshold == 0 {
		c.Volatile.DefaultRS.MaxThreshold = 95
	}
	if c.Volatile.MaxBufferMem.Int() == 0 {
		c.Volatile.MaxBufferMem.Set("4MiB")
	}
	if c.Volatile.ErasureShareSize.Int() == 0 {
		c.Volatile.ErasureShareSize.Set("1KiB")
	}
	if c.Volatile.SegmentSize.Int() == 0 {
		c.Volatile.SegmentSize.Set("64MiB")
	}
	if c.Volatile.MaxInlineSize.Int() == 0 {
		c.Volatile.MaxInlineSize.Set("4KiB")
	}
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess information
func (p *Project) OpenBucket(ctx context.Context, bucket string, cfg BucketConfig) (b *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	cfg.setDefaults()

	bucketInfo, err := p.GetBucketInfo(ctx, bucket)
	if err != nil {
		return nil, err
	}

	if cfg.Volatile.ErasureShareSize.Int()*cfg.Volatile.DefaultRS.MinThreshold%cfg.Volatile.EncryptionBlockSize.Int() != 0 {
		return nil, Error.New("EncryptionBlockSize must be a multiple of ErasureShareSize * RS MinThreshold")
	}
	if cfg.EncryptionAccess.Key == (storj.Key{}) {
		return nil, Error.New("No encryption key chosen")
	}
	pathCipher, err := cfg.Volatile.PathCipher.convert()
	if err != nil {
		return nil, err
	}
	dataCipher, err := cfg.Volatile.DataCipher.convert()
	if err != nil {
		return nil, err
	}

	ec := ecclient.NewClient(p.tc, cfg.Volatile.MaxBufferMem.Int())
	fc, err := infectious.NewFEC(cfg.Volatile.DefaultRS.MinThreshold, cfg.Volatile.DefaultRS.MaxThreshold)
	if err != nil {
		return nil, err
	}
	rs, err := eestream.NewRedundancyStrategy(
		eestream.NewRSScheme(fc, cfg.Volatile.ErasureShareSize.Int()),
		cfg.Volatile.DefaultRS.RepairThreshold,
		cfg.Volatile.DefaultRS.SuccessThreshold)
	if err != nil {
		return nil, err
	}

	maxEncryptedSegmentSize, err := encryption.CalcEncryptedSize(cfg.Volatile.SegmentSize.Int64(),
		storj.EncryptionScheme{
			Cipher:    dataCipher,
			BlockSize: int32(cfg.Volatile.EncryptionBlockSize.Int()),
		})
	if err != nil {
		return nil, err
	}
	segments := segments.NewSegmentStore(p.metainfo, ec, rs, cfg.Volatile.MaxInlineSize.Int(), maxEncryptedSegmentSize)

	key := new(storj.Key)
	copy(key[:], cfg.EncryptionAccess.Key[:])

	streams, err := streams.NewStreamStore(segments, cfg.Volatile.SegmentSize.Int64(), key, cfg.Volatile.EncryptionBlockSize.Int(), dataCipher)
	if err != nil {
		return nil, err
	}

	buckets := buckets.NewStore(streams)

	return &Bucket{
		Bucket:     bucketInfo,
		metainfo:   kvmetainfo.New(p.metainfo, buckets, streams, segments, key),
		streams:    streams,
		pathCipher: pathCipher,
	}, nil
}

// Close closes the Project
func (p *Project) Close() error {
	return nil
}
