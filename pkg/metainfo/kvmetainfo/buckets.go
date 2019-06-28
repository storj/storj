// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"bytes"
	"context"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// CreateBucket creates a new bucket or updates and existing bucket with the specified information
func (db *Project) CreateBucket(ctx context.Context, bucketName string, info *storj.Bucket) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}
	if info == nil {
		info = &storj.Bucket{PathCipher: storj.AESGCM}
	}
	if info.EncryptionParameters.CipherSuite == storj.EncUnspecified {
		info.EncryptionParameters.CipherSuite = storj.EncAESGCM
	}
	if info.EncryptionParameters.BlockSize == 0 {
		info.EncryptionParameters.BlockSize = db.encryptedBlockSize
	}
	if info.RedundancyScheme.Algorithm == storj.InvalidRedundancyAlgorithm {
		info.RedundancyScheme.Algorithm = storj.ReedSolomon
	}
	if info.RedundancyScheme.RequiredShares == 0 {
		info.RedundancyScheme.RequiredShares = int16(db.redundancy.RequiredCount())
	}
	if info.RedundancyScheme.RepairShares == 0 {
		info.RedundancyScheme.RepairShares = int16(db.redundancy.RepairThreshold())
	}
	if info.RedundancyScheme.OptimalShares == 0 {
		info.RedundancyScheme.OptimalShares = int16(db.redundancy.OptimalThreshold())
	}
	if info.RedundancyScheme.TotalShares == 0 {
		info.RedundancyScheme.TotalShares = int16(db.redundancy.TotalCount())
	}
	if info.RedundancyScheme.ShareSize == 0 {
		info.RedundancyScheme.ShareSize = int32(db.redundancy.ErasureShareSize())
	}
	if info.SegmentsSize == 0 {
		info.SegmentsSize = db.segmentsSize
	}

	if err := validateBlockSize(info.RedundancyScheme, info.EncryptionParameters.BlockSize); err != nil {
		return bucketInfo, err
	}

	if info.PathCipher < storj.Unencrypted || info.PathCipher > storj.SecretBox {
		return storj.Bucket{}, encryption.ErrInvalidConfig.New("encryption type %d is not supported", info.PathCipher)
	}

	r := bytes.NewReader(nil)
	userMeta := map[string]string{
		"attribution-to":    info.Attribution,
		"path-enc-type":     strconv.Itoa(int(info.PathCipher)),
		"default-seg-size":  strconv.FormatInt(info.SegmentsSize, 10),
		"default-enc-type":  strconv.Itoa(int(info.EncryptionParameters.CipherSuite.ToCipher())),
		"default-enc-blksz": strconv.FormatInt(int64(info.EncryptionParameters.BlockSize), 10),
		"default-rs-algo":   strconv.Itoa(int(info.RedundancyScheme.Algorithm)),
		"default-rs-sharsz": strconv.FormatInt(int64(info.RedundancyScheme.ShareSize), 10),
		"default-rs-reqd":   strconv.Itoa(int(info.RedundancyScheme.RequiredShares)),
		"default-rs-repair": strconv.Itoa(int(info.RedundancyScheme.RepairShares)),
		"default-rs-optim":  strconv.Itoa(int(info.RedundancyScheme.OptimalShares)),
		"default-rs-total":  strconv.Itoa(int(info.RedundancyScheme.TotalShares)),
	}
	var exp time.Time
	m, err := db.buckets.Put(ctx, bucketName, r, pb.SerializableMeta{UserDefined: userMeta}, exp)
	if err != nil {
		return storj.Bucket{}, err
	}

	rv := *info
	rv.Name = bucketName
	rv.Created = m.Modified
	return rv, nil
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

	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrBucketNotFound.Wrap(err)
	}

	return err
}

// GetBucket gets bucket information
func (db *Project) GetBucket(ctx context.Context, bucketName string) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	objMeta, err := db.buckets.Meta(ctx, bucketName)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, err
	}

	return bucketFromMeta(ctx, bucketName, objMeta)
}

// ListBuckets lists buckets
func (db *Project) ListBuckets(ctx context.Context, options storj.BucketListOptions) (list storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	var startAfter, endBefore string
	switch options.Direction {
	case storj.Before:
		// before lists backwards from cursor, without cursor
		endBefore = options.Cursor
	case storj.Backward:
		// backward lists backwards from cursor, including cursor
		endBefore = keyAfter(options.Cursor)
	case storj.Forward:
		// forward lists forwards from cursor, including cursor
		startAfter = keyBefore(options.Cursor)
	case storj.After:
		// after lists forwards from cursor, without cursor
		startAfter = options.Cursor
	default:
		return storj.BucketList{}, errClass.New("invalid direction %d", options.Direction)
	}

	// TODO: remove this hack-fix of specifying the last key
	if options.Cursor == "" && (options.Direction == storj.Before || options.Direction == storj.Backward) {
		endBefore = "\x7f\x7f\x7f\x7f\x7f\x7f\x7f"
	}

	objItems, more, err := db.buckets.List(ctx, "", startAfter, endBefore, false, options.Limit, meta.Modified)
	if err != nil {
		return storj.BucketList{}, err
	}

	list = storj.BucketList{
		More:  more,
		Items: make([]storj.Bucket, 0, len(objItems)),
	}

	for _, itm := range objItems {
		if itm.IsPrefix {
			continue
		}
		m, err := bucketFromMeta(ctx, itm.Path, itm.Meta)
		if err != nil {
			return storj.BucketList{}, err
		}
		list.Items = append(list.Items, m)
	}

	return list, nil
}

func bucketFromMeta(ctx context.Context, bucketName string, m objects.Meta) (out storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	out.Name = bucketName
	out.Created = m.Modified
	// backwards compatibility for old buckets
	out.PathCipher = storj.AESGCM
	out.EncryptionParameters.CipherSuite = storj.EncUnspecified

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

	es := &out.EncryptionParameters
	rs := &out.RedundancyScheme

	out.Attribution = m.UserDefined["attribution-to"]
	applySetting("path-enc-type", 16, func(v int64) { out.PathCipher = storj.Cipher(v) })
	applySetting("default-seg-size", 64, func(v int64) { out.SegmentsSize = v })
	applySetting("default-enc-type", 32, func(v int64) { es.CipherSuite = storj.Cipher(v).ToCipherSuite() })
	applySetting("default-enc-blksz", 32, func(v int64) { es.BlockSize = int32(v) })
	applySetting("default-rs-algo", 32, func(v int64) { rs.Algorithm = storj.RedundancyAlgorithm(v) })
	applySetting("default-rs-sharsz", 32, func(v int64) { rs.ShareSize = int32(v) })
	applySetting("default-rs-reqd", 16, func(v int64) { rs.RequiredShares = int16(v) })
	applySetting("default-rs-repair", 16, func(v int64) { rs.RepairShares = int16(v) })
	applySetting("default-rs-optim", 16, func(v int64) { rs.OptimalShares = int16(v) })
	applySetting("default-rs-total", 16, func(v int64) { rs.TotalShares = int16(v) })

	return out, err
}
