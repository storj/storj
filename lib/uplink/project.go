// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
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
	metainfo      *metainfo.Client
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
		// SegmentsSize is the default segment size to use for new
		// objects in this Bucket.
		SegmentsSize memory.Size
	}
}

func (cfg *BucketConfig) clone() *BucketConfig {
	clone := *cfg
	return &clone
}

// TODO: is this the best way to do this?
func (cfg *BucketConfig) setDefaults() {
	if cfg.PathCipher == storj.EncUnspecified {
		cfg.PathCipher = defaultCipher
	}
	if cfg.EncryptionParameters.CipherSuite == storj.EncUnspecified {
		cfg.EncryptionParameters.CipherSuite = defaultCipher
	}
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
		cfg.Volatile.RedundancyScheme.TotalShares = 130
	}
	if cfg.Volatile.RedundancyScheme.ShareSize == 0 {
		cfg.Volatile.RedundancyScheme.ShareSize = 256 * memory.B.Int32()
	}
	if cfg.EncryptionParameters.BlockSize == 0 {
		cfg.EncryptionParameters.BlockSize = cfg.Volatile.RedundancyScheme.ShareSize * int32(cfg.Volatile.RedundancyScheme.RequiredShares)
	}
	if cfg.Volatile.SegmentsSize.Int() == 0 {
		cfg.Volatile.SegmentsSize = 64 * memory.MiB
	}
}

// CreateBucket creates a new bucket if authorized.
func (p *Project) CreateBucket(ctx context.Context, name string, cfg *BucketConfig) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	if cfg == nil {
		cfg = &BucketConfig{}
	}
	cfg = cfg.clone()
	cfg.setDefaults()

	bucket = storj.Bucket{
		PathCipher:           cfg.PathCipher.ToCipher(),
		EncryptionParameters: cfg.EncryptionParameters,
		RedundancyScheme:     cfg.Volatile.RedundancyScheme,
		SegmentsSize:         cfg.Volatile.SegmentsSize.Int64(),
	}
	return p.project.CreateBucket(ctx, name, &bucket)
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
		opts = &BucketListOptions{Direction: storj.Forward}
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
	cfg := &BucketConfig{
		PathCipher:           b.PathCipher.ToCipherSuite(),
		EncryptionParameters: b.EncryptionParameters,
	}
	cfg.Volatile.RedundancyScheme = b.RedundancyScheme
	cfg.Volatile.SegmentsSize = memory.Size(b.SegmentsSize)
	return b, cfg, nil
}

// TODO: move the bucket related OpenBucket to bucket.go

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
func (p *Project) OpenBucket(ctx context.Context, bucketName string, access *EncryptionAccess) (b *Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketInfo, cfg, err := p.GetBucketInfo(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	// partnerID set and bucket's attribution is not set
	if p.uplinkCfg.Volatile.PartnerID != "" && bucketInfo.Attribution == "" {
		err = p.checkBucketAttribution(ctx, bucketName)
		if err != nil {
			return nil, err
		}

		// update the bucket with attribution info
		bucketInfo, err = p.updateBucket(ctx, bucketInfo)
		if err != nil {
			return nil, err
		}
	}

	encryptionScheme := cfg.EncryptionParameters.ToEncryptionScheme()

	ec := ecclient.NewClient(p.uplinkCfg.Volatile.Log.Named("ecclient"), p.tc, p.uplinkCfg.Volatile.MaxMemory.Int())
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

	maxEncryptedSegmentSize, err := encryption.CalcEncryptedSize(cfg.Volatile.SegmentsSize.Int64(),
		cfg.EncryptionParameters.ToEncryptionScheme())
	if err != nil {
		return nil, err
	}
	segmentStore := segments.NewSegmentStore(p.metainfo, ec, rs, p.maxInlineSize.Int(), maxEncryptedSegmentSize)

	streamStore, err := streams.NewStreamStore(segmentStore, cfg.Volatile.SegmentsSize.Int64(), access.store, int(encryptionScheme.BlockSize), encryptionScheme.Cipher, p.maxInlineSize.Int())
	if err != nil {
		return nil, err
	}

	return &Bucket{
		BucketConfig: *cfg,
		Name:         bucketInfo.Name,
		Created:      bucketInfo.Created,
		bucket:       bucketInfo,
		metainfo:     kvmetainfo.New(p.project, p.metainfo, streamStore, segmentStore, access.store),
		streams:      streamStore,
	}, nil
}

func (p *Project) retrieveSalt(ctx context.Context) (salt []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := p.metainfo.GetProjectInfo(ctx)
	if err != nil {
		return nil, err
	}
	return info.ProjectSalt, nil
}

// SaltedKeyFromPassphrase returns a key generated from the given passphrase using a stable, project-specific salt
func (p *Project) SaltedKeyFromPassphrase(ctx context.Context, passphrase string) (_ *storj.Key, err error) {
	defer mon.Task()(&ctx)(&err)
	salt, err := p.retrieveSalt(ctx)
	if err != nil {
		return nil, err
	}
	key, err := encryption.DeriveDefaultPassword([]byte(passphrase), salt)
	if err != nil {
		return nil, err
	}
	if len(key) != len(storj.Key{}) {
		return nil, errs.New("unexpected key length!")
	}
	var result storj.Key
	copy(result[:], key)
	return &result, nil
}

// checkBucketAttribution Checks the bucket attribution
func (p *Project) checkBucketAttribution(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if p.uplinkCfg.Volatile.PartnerID == "" {
		return nil
	}

	partnerID, err := uuid.Parse(p.uplinkCfg.Volatile.PartnerID)
	if err != nil {
		return Error.Wrap(err)
	}

	return p.metainfo.SetAttribution(ctx, bucketName, *partnerID)
}

// updateBucket updates an existing bucket's attribution info.
func (p *Project) updateBucket(ctx context.Context, bucketInfo storj.Bucket) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket = storj.Bucket{
		Attribution:          p.uplinkCfg.Volatile.PartnerID,
		PathCipher:           bucketInfo.PathCipher,
		EncryptionParameters: bucketInfo.EncryptionParameters,
		RedundancyScheme:     bucketInfo.RedundancyScheme,
		SegmentsSize:         bucketInfo.SegmentsSize,
	}
	return p.project.CreateBucket(ctx, bucketInfo.Name, &bucket)
}
