// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"bytes"
	"context"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var mon = monkit.Package()

// Store creates an interface for interacting with buckets
type Store interface {
	Get(ctx context.Context, bucket string) (meta Meta, err error)
	Put(ctx context.Context, bucket string, inMeta Meta) (meta Meta, err error)
	Delete(ctx context.Context, bucket string) (err error)
	List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error)
	GetObjectStore(ctx context.Context, bucketName string) (store objects.Store, err error)
}

// ListItem is a single item in a listing
type ListItem struct {
	Bucket string
	Meta   Meta
}

// BucketStore contains objects store
type BucketStore struct {
	store  objects.Store
	stream streams.Store
}

// Meta is the bucket metadata struct
type Meta struct {
	Created            time.Time
	PathEncryptionType storj.Cipher
	SegmentsSize       int64
	RedundancyScheme   storj.RedundancyScheme
	EncryptionScheme   storj.EncryptionScheme
}

// NewStore instantiates BucketStore
func NewStore(stream streams.Store) Store {
	// root object store for storing the buckets with unencrypted names
	store := objects.NewStore(stream, storj.Unencrypted)
	return &BucketStore{store: store, stream: stream}
}

// GetObjectStore returns an implementation of objects.Store
func (b *BucketStore) GetObjectStore(ctx context.Context, bucket string) (objects.Store, error) {
	if bucket == "" {
		return nil, storj.ErrNoBucket.New("")
	}

	m, err := b.Get(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return nil, err
	}
	prefixed := prefixedObjStore{
		store:  objects.NewStore(b.stream, m.PathEncryptionType),
		prefix: bucket,
	}
	return &prefixed, nil
}

// Get calls objects store Get
func (b *BucketStore) Get(ctx context.Context, bucket string) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return Meta{}, storj.ErrNoBucket.New("")
	}

	objMeta, err := b.store.Meta(ctx, bucket)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			err = storj.ErrBucketNotFound.Wrap(err)
		}
		return Meta{}, err
	}

	return convertMeta(objMeta)
}

// Put calls objects store Put and fills in some specific metadata to be used
// in the bucket's object Pointer. Note that the Meta.Created field is ignored.
func (b *BucketStore) Put(ctx context.Context, bucketName string, inMeta Meta) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return Meta{}, storj.ErrNoBucket.New("")
	}

	pathCipher := inMeta.PathEncryptionType
	if pathCipher < storj.Unencrypted || pathCipher > storj.SecretBox {
		return Meta{}, encryption.ErrInvalidConfig.New("encryption type %d is not supported", pathCipher)
	}

	r := bytes.NewReader(nil)
	userMeta := map[string]string{
		"path-enc-type":     strconv.Itoa(int(pathCipher)),
		"default-seg-size":  strconv.FormatInt(inMeta.SegmentsSize, 10),
		"default-enc-type":  strconv.Itoa(int(inMeta.EncryptionScheme.Cipher)),
		"default-enc-blksz": strconv.FormatInt(int64(inMeta.EncryptionScheme.BlockSize), 10),
		"default-rs-algo":   strconv.Itoa(int(inMeta.RedundancyScheme.Algorithm)),
		"default-rs-sharsz": strconv.FormatInt(int64(inMeta.RedundancyScheme.ShareSize), 10),
		"default-rs-reqd":   strconv.Itoa(int(inMeta.RedundancyScheme.RequiredShares)),
		"default-rs-repair": strconv.Itoa(int(inMeta.RedundancyScheme.RepairShares)),
		"default-rs-optim":  strconv.Itoa(int(inMeta.RedundancyScheme.OptimalShares)),
		"default-rs-total":  strconv.Itoa(int(inMeta.RedundancyScheme.TotalShares)),
	}
	var exp time.Time
	m, err := b.store.Put(ctx, bucketName, r, pb.SerializableMeta{UserDefined: userMeta}, exp)
	if err != nil {
		return Meta{}, err
	}
	// we could use convertMeta() here, but that's a lot of int-parsing
	// just to get back to what should be the same contents we already
	// have. the only change ought to be the modified time.
	inMeta.Created = m.Modified
	return inMeta, nil
}

// Delete calls objects store Delete
func (b *BucketStore) Delete(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.ErrNoBucket.New("")
	}

	err = b.store.Delete(ctx, bucket)

	if storage.ErrKeyNotFound.Has(err) {
		err = storj.ErrBucketNotFound.Wrap(err)
	}

	return err
}

// List calls objects store List
func (b *BucketStore) List(ctx context.Context, startAfter, endBefore string, limit int) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	objItems, more, err := b.store.List(ctx, "", startAfter, endBefore, false, limit, meta.Modified)
	if err != nil {
		return items, more, err
	}

	items = make([]ListItem, 0, len(objItems))
	for _, itm := range objItems {
		if itm.IsPrefix {
			continue
		}
		m, err := convertMeta(itm.Meta)
		if err != nil {
			return items, more, err
		}
		items = append(items, ListItem{
			Bucket: itm.Path,
			Meta:   m,
		})
	}
	return items, more, nil
}

// convertMeta converts stream metadata to object metadata
func convertMeta(m objects.Meta) (out Meta, err error) {
	out.Created = m.Modified
	// backwards compatibility for old buckets
	out.PathEncryptionType = storj.AESGCM
	out.EncryptionScheme.Cipher = storj.Invalid

	if pathEncType := m.UserDefined["path-enc-type"]; pathEncType != "" {
		pet, err := strconv.Atoi(pathEncType)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for path-enc-type: %v", err)
		}
		out.PathEncryptionType = storj.Cipher(pet)
	}
	if defaultSegSize := m.UserDefined["default-seg-size"]; defaultSegSize != "" {
		dss, err := strconv.ParseInt(defaultSegSize, 10, 64)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for path-enc-type: %v", err)
		}
		out.SegmentsSize = dss
	}
	if defaultEncType := m.UserDefined["default-enc-type"]; defaultEncType != "" {
		det, err := strconv.Atoi(defaultEncType)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-enc-type: %v", err)
		}
		out.EncryptionScheme.Cipher = storj.Cipher(det)
	}
	if defaultEncBlockSize := m.UserDefined["default-enc-blksz"]; defaultEncBlockSize != "" {
		deb, err := strconv.ParseInt(defaultEncBlockSize, 10, 32)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-enc-blksz: %v", err)
		}
		out.EncryptionScheme.BlockSize = int32(deb)
	}
	if defaultRSAlgo := m.UserDefined["default-rs-algo"]; defaultRSAlgo != "" {
		dra, err := strconv.Atoi(defaultRSAlgo)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-rs-algo: %v", err)
		}
		out.RedundancyScheme.Algorithm = storj.RedundancyAlgorithm(dra)
	}
	if defaultRSShareSize := m.UserDefined["default-rs-sharsz"]; defaultRSShareSize != "" {
		drs, err := strconv.ParseInt(defaultRSShareSize, 10, 32)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-rs-sharsz: %v", err)
		}
		out.RedundancyScheme.ShareSize = int32(drs)
	}
	if defaultRSRequired := m.UserDefined["default-rs-reqd"]; defaultRSRequired != "" {
		drr, err := strconv.ParseInt(defaultRSRequired, 10, 16)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-rs-reqd: %v", err)
		}
		out.RedundancyScheme.RepairShares = int16(drr)
	}
	if defaultRSRepairThresh := m.UserDefined["default-rs-repair"]; defaultRSRepairThresh != "" {
		drr, err := strconv.ParseInt(defaultRSRepairThresh, 10, 16)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-rs-repair: %v", err)
		}
		out.RedundancyScheme.RepairShares = int16(drr)
	}
	if defaultRSOptimal := m.UserDefined["default-rs-optim"]; defaultRSOptimal != "" {
		drr, err := strconv.ParseInt(defaultRSOptimal, 10, 16)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-rs-optim: %v", err)
		}
		out.RedundancyScheme.RepairShares = int16(drr)
	}
	if defaultRSTotal := m.UserDefined["default-rs-total"]; defaultRSTotal != "" {
		drr, err := strconv.ParseInt(defaultRSTotal, 10, 16)
		if err != nil {
			return Meta{}, errs.New("invalid metadata field for default-rs-total: %v", err)
		}
		out.RedundancyScheme.RepairShares = int16(drr)
	}

	return out, nil
}
