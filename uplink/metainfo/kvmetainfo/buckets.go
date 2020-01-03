// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/storj/uplink/metainfo"
)

// CreateBucket creates a new bucket
func (db *Project) CreateBucket(ctx context.Context, bucketName string, info *storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}
	if info == nil {
		info = &storj.Bucket{PathCipher: storj.EncAESGCM}
	}
	if info.DefaultEncryptionParameters.CipherSuite == storj.EncUnspecified {
		info.DefaultEncryptionParameters.CipherSuite = storj.EncAESGCM
	}
	if info.DefaultEncryptionParameters.BlockSize == 0 {
		info.DefaultEncryptionParameters.BlockSize = db.encryptedBlockSize
	}
	if info.DefaultRedundancyScheme.Algorithm == storj.InvalidRedundancyAlgorithm {
		info.DefaultRedundancyScheme.Algorithm = storj.ReedSolomon
	}
	if info.DefaultRedundancyScheme.RequiredShares == 0 {
		info.DefaultRedundancyScheme.RequiredShares = int16(db.redundancy.RequiredCount())
	}
	if info.DefaultRedundancyScheme.RepairShares == 0 {
		info.DefaultRedundancyScheme.RepairShares = int16(db.redundancy.RepairThreshold())
	}
	if info.DefaultRedundancyScheme.OptimalShares == 0 {
		info.DefaultRedundancyScheme.OptimalShares = int16(db.redundancy.OptimalThreshold())
	}
	if info.DefaultRedundancyScheme.TotalShares == 0 {
		info.DefaultRedundancyScheme.TotalShares = int16(db.redundancy.TotalCount())
	}
	if info.DefaultRedundancyScheme.ShareSize == 0 {
		info.DefaultRedundancyScheme.ShareSize = int32(db.redundancy.ErasureShareSize())
	}
	if info.DefaultSegmentsSize == 0 {
		info.DefaultSegmentsSize = db.segmentsSize
	}

	if err := validateBlockSize(info.DefaultRedundancyScheme, info.DefaultEncryptionParameters.BlockSize); err != nil {
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}

	if info.PathCipher < storj.EncNull || info.PathCipher > storj.EncSecretBox {
		return storj.Bucket{}, encryption.ErrInvalidConfig.New("encryption type %d is not supported", info.PathCipher)
	}

	info.Name = bucketName

	// uuid MarshalJSON implementation always returns err == nil
	partnerID, _ := info.PartnerID.MarshalJSON()
	newBucket, err := db.metainfo.CreateBucket(ctx, metainfo.CreateBucketParams{
		Name:                        []byte(info.Name),
		PathCipher:                  info.PathCipher,
		PartnerID:                   partnerID,
		DefaultSegmentsSize:         info.DefaultSegmentsSize,
		DefaultRedundancyScheme:     info.DefaultRedundancyScheme,
		DefaultEncryptionParameters: info.DefaultEncryptionParameters,
	})
	if err != nil {
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}

	return newBucket, nil
}

// validateBlockSize confirms the encryption block size aligns with stripe size.
// Stripes contain encrypted data therefore we want the stripe boundaries to match
// with the encryption block size boundaries. We also want stripes to be small for
// audits, but encryption can be a bit larger. All told, block size should be an integer
// multiple of stripe size.
func validateBlockSize(redundancyScheme storj.RedundancyScheme, blockSize int32) error {
	stripeSize := redundancyScheme.StripeSize()

	if blockSize%stripeSize != 0 {
		return errs.New("encryption BlockSize (%d) must be a multiple of RS ShareSize (%d) * RS RequiredShares (%d)",
			blockSize, redundancyScheme.ShareSize, redundancyScheme.RequiredShares,
		)
	}
	return nil
}

// DeleteBucket deletes bucket
func (db *Project) DeleteBucket(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.ErrNoBucket.New("")
	}
	err = db.metainfo.DeleteBucket(ctx, metainfo.DeleteBucketParams{
		Name: []byte(bucketName),
	})
	if err != nil {
		return storj.ErrBucket.Wrap(err)
	}

	return nil
}

// GetBucket gets bucket information
func (db *Project) GetBucket(ctx context.Context, bucketName string) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	bucket, err := db.metainfo.GetBucket(ctx, metainfo.GetBucketParams{
		Name: []byte(bucketName),
	})
	if err != nil {
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}

	return bucket, nil
}

// ListBuckets lists buckets
func (db *Project) ListBuckets(ctx context.Context, listOpts storj.BucketListOptions) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketList, err := db.metainfo.ListBuckets(ctx, metainfo.ListBucketsParams{
		ListOpts: listOpts,
	})
	if err != nil {
		return storj.BucketList{}, storj.ErrBucket.Wrap(err)
	}

	return bucketList, nil
}
