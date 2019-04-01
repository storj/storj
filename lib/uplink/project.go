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

// CreateBucketOptions holds possible options that can be passed to
// CreateBucket.
// TODO(paul): shouldn't this be like BucketOptions (see paul TODOs below)?
type CreateBucketOptions struct {
	// PathCipher indicates which ciphersuite is to be used for path
	// encryption within the new Bucket. If not set, AES-GCM encryption
	// will be used.
	PathCipher storj.CipherSuite
}

func (o *CreateBucketOptions) setDefaults() {
	if o.PathCipher == storj.EncUnspecified {
		o.PathCipher = defaultCipher
	}
}

// CreateBucket creates a new bucket if authorized.
func (p *Project) CreateBucket(ctx context.Context, bucket string, opts *CreateBucketOptions) (b storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts == nil {
		opts = &CreateBucketOptions{}
	}
	opts.setDefaults()
	pathCipher := opts.PathCipher.ToCipher()
	return p.project.CreateBucket(ctx, bucket, &storj.Bucket{PathCipher: pathCipher})
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
func (p *Project) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return p.project.DeleteBucket(ctx, bucket)
}

// ListBuckets will list authorized buckets.
func (p *Project) ListBuckets(ctx context.Context, opts *storj.BucketListOptions) (bl storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts == nil {
		opts = &storj.BucketListOptions{}
	}
	return p.project.ListBuckets(ctx, *opts)
}

// GetBucketInfo returns info about the requested bucket if authorized.
func (p *Project) GetBucketInfo(ctx context.Context, bucket string) (b storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return p.project.GetBucket(ctx, bucket)
}

// BucketConfig represents configuration options for a specific Bucket
// TODO(paul): doesn't this make more sense with CreateBucket? Why do we have
//             Pointer objects for buckets if we aren't using them for this?
type BucketConfig struct {
	// EncryptionAccess specifies the encryption details needed to
	// encrypt/decrypt objects within this Bucket.
	EncryptionAccess EncryptionAccess

	// PathCipher specifies the ciphersuite to be used for path encryption
	// in this Bucket.
	PathCipher storj.CipherSuite

	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// DefaultRS defines the default Reed-Solomon and/or Forward
		// Error Correction encoding parameters to be used by objects
		// in this Bucket.
		DefaultRS storj.RedundancyScheme
		// MaxBufferMem is the default maximum amount of memory to be
		// allocated for read buffers while performing decodes of
		// objects in this Bucket. If set to a negative value, the
		// system will use the smallest amount of memory it can.
		MaxBufferMem memory.Size
		// SegmentSize is the default segment size to use for new
		// objects in this Bucket.
		SegmentSize memory.Size
		// MaxInlineSize determines whether the uplink will attempt to
		// store a new object in the satellite's pointerDB. Objects at
		// or below this size will be marked for inline storage, and
		// objects above this size will not. (The satellite may reject
		// the inline storage and require remote storage, still.)
		MaxInlineSize memory.Size

		// EncryptionParameters specifies the default encryption
		// parameters to be used for data encryption of new Objects in
		// this bucket.
		EncryptionParameters storj.EncryptionParameters
	}
}

func (c *BucketConfig) setDefaults() {
	if c.PathCipher == storj.EncUnspecified {
		c.PathCipher = defaultCipher
	}
	if c.Volatile.EncryptionParameters.CipherSuite == storj.EncUnspecified {
		c.Volatile.EncryptionParameters.CipherSuite = defaultCipher
	}
	if c.Volatile.EncryptionParameters.BlockSize == 0 {
		c.Volatile.EncryptionParameters.BlockSize = (1 * memory.KiB).Int32()
	}
	if c.Volatile.DefaultRS.RequiredShares == 0 {
		c.Volatile.DefaultRS.RequiredShares = 29
	}
	if c.Volatile.DefaultRS.RepairShares == 0 {
		c.Volatile.DefaultRS.RepairShares = 35
	}
	if c.Volatile.DefaultRS.OptimalShares == 0 {
		c.Volatile.DefaultRS.OptimalShares = 80
	}
	if c.Volatile.DefaultRS.TotalShares == 0 {
		c.Volatile.DefaultRS.TotalShares = 95
	}
	if c.Volatile.MaxBufferMem.Int() == 0 {
		c.Volatile.MaxBufferMem = 4 * memory.MiB
	} else if c.Volatile.MaxBufferMem.Int() < 0 {
		c.Volatile.MaxBufferMem = 0
	}
	if c.Volatile.DefaultRS.ShareSize == 0 {
		c.Volatile.DefaultRS.ShareSize = (1 * memory.KiB).Int32()
	}
	if c.Volatile.SegmentSize.Int() == 0 {
		c.Volatile.SegmentSize = 64 * memory.MiB
	}
	if c.Volatile.MaxInlineSize.Int() == 0 {
		c.Volatile.MaxInlineSize = 4 * memory.KiB
	}
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
// TODO(paul): can't all this bucket config info come from the bucket metadata in pdb?
//             why are we passing it in? what happens if it doesn't match?
func (p *Project) OpenBucket(ctx context.Context, bucket string, cfg *BucketConfig) (b *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	cfg.setDefaults()

	if cfg == nil {
		cfg = &BucketConfig{}
	}
	bucketInfo, err := p.GetBucketInfo(ctx, bucket)
	if err != nil {
		return nil, err
	}

	if cfg.Volatile.DefaultRS.ShareSize*int32(cfg.Volatile.DefaultRS.RequiredShares)%cfg.Volatile.EncryptionParameters.BlockSize != 0 {
		return nil, Error.New("EncryptionParameters.BlockSize must be a multiple of RS ShareSize * RS RequiredShares")
	}
	if cfg.EncryptionAccess.Key == (storj.Key{}) {
		return nil, Error.New("No encryption key chosen")
	}
	pathCipher := cfg.PathCipher.ToCipher()
	if err != nil {
		return nil, err
	}
	encryptionScheme := cfg.Volatile.EncryptionParameters.ToEncryptionScheme()

	ec := ecclient.NewClient(p.tc, cfg.Volatile.MaxBufferMem.Int())
	fc, err := infectious.NewFEC(int(cfg.Volatile.DefaultRS.RequiredShares), int(cfg.Volatile.DefaultRS.TotalShares))
	if err != nil {
		return nil, err
	}
	rs, err := eestream.NewRedundancyStrategy(
		eestream.NewRSScheme(fc, int(cfg.Volatile.DefaultRS.ShareSize)),
		int(cfg.Volatile.DefaultRS.RepairShares),
		int(cfg.Volatile.DefaultRS.OptimalShares))
	if err != nil {
		return nil, err
	}

	maxEncryptedSegmentSize, err := encryption.CalcEncryptedSize(cfg.Volatile.SegmentSize.Int64(),
		cfg.Volatile.EncryptionParameters.ToEncryptionScheme())
	if err != nil {
		return nil, err
	}
	segments := segments.NewSegmentStore(p.metainfo, ec, rs, cfg.Volatile.MaxInlineSize.Int(), maxEncryptedSegmentSize)

	key := new(storj.Key)
	copy(key[:], cfg.EncryptionAccess.Key[:])

	streams, err := streams.NewStreamStore(segments, cfg.Volatile.SegmentSize.Int64(), key, int(encryptionScheme.BlockSize), encryptionScheme.Cipher)
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

// Close closes the Project.
func (p *Project) Close() error {
	return nil
}
