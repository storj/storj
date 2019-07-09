// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"strconv"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// CreateBucket creates a new bucket or updates and existing bucket with the specified information
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
		return storj.Bucket{}, err
	}

	if info.PathCipher < storj.EncNull || info.PathCipher > storj.EncSecretBox {
		return storj.Bucket{}, encryption.ErrInvalidConfig.New("encryption type %d is not supported", info.PathCipher)
	}

	info.Name = bucketName
	newBucket, err := db.buckets.Create(ctx, *info)
	if err != nil {
		return storj.Bucket{}, err
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

	err = db.buckets.Delete(ctx, bucketName)
	if err != nil {
		return err
	}

	return err
}

// GetBucket gets bucket information
func (db *Project) GetBucket(ctx context.Context, bucketName string) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	bucket, err := db.buckets.Get(ctx, bucketName)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, err
	}

	return bucket, err
}

// ListBuckets lists buckets
func (db *Project) ListBuckets(ctx context.Context, listOpts storj.BucketListOptions) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	bucketList, err := db.buckets.List(ctx, listOpts)
	if err != nil {
		return storj.BucketList{}, err
	}

	return bucketList, nil
}

func bucketFromMeta(ctx context.Context, bucketName string, m objects.Meta) (out storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	out.Name = bucketName
	out.Created = m.Modified

	applySetting := func(nameInMap string, bits int, storeFunc func(val int64)) {
		if err != nil {
			return
		}
		if stringVal := m.UserDefined[nameInMap]; stringVal != "" {
			var intVal int64
			intVal, err = strconv.ParseInt(stringVal, 10, bits)
			if err != nil {
				err = errs.New("invalid metadata field for %s: %v", nameInMap, err)
				return
			}
			storeFunc(intVal)
		}
	}

	es := &out.DefaultEncryptionParameters
	rs := &out.DefaultRedundancyScheme

	out.Attribution = m.UserDefined["attribution-to"]
	applySetting("path-enc-type", 16, func(v int64) { out.PathCipher = storj.CipherSuite(v) })
	applySetting("default-seg-size", 64, func(v int64) { out.DefaultSegmentsSize = v })
	applySetting("default-enc-type", 32, func(v int64) { es.CipherSuite = storj.CipherSuite(v) })
	applySetting("default-enc-blksz", 32, func(v int64) { es.BlockSize = int32(v) })
	applySetting("default-rs-algo", 32, func(v int64) { rs.Algorithm = storj.RedundancyAlgorithm(v) })
	applySetting("default-rs-sharsz", 32, func(v int64) { rs.ShareSize = int32(v) })
	applySetting("default-rs-reqd", 16, func(v int64) { rs.RequiredShares = int16(v) })
	applySetting("default-rs-repair", 16, func(v int64) { rs.RepairShares = int16(v) })
	applySetting("default-rs-optim", 16, func(v int64) { rs.OptimalShares = int16(v) })
	applySetting("default-rs-total", 16, func(v int64) { rs.TotalShares = int16(v) })

	return out, err
}
