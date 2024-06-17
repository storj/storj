// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package statcache

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/storagenode/blobstore"
)

// Cache can store file metadata (size/mod time) and make it available (quickly).
type Cache interface {
	Get(ctx context.Context, namespace []byte, key []byte) (blobstore.FileInfo, bool, error)
	Set(ctx context.Context, namespace []byte, key []byte, value blobstore.FileInfo) error
	Delete(ctx context.Context, namespace []byte, key []byte) error
	Close() error
}

// CachedStatBlobstore implements a blob store, but also manages an external cache for file metadata.
type CachedStatBlobstore struct {
	blobstore.Blobs
	cache Cache
	log   *zap.Logger
}

var _ blobstore.Blobs = &CachedStatBlobstore{}

// NewCachedStatBlobStore creates a new cached blobstore.
func NewCachedStatBlobStore(log *zap.Logger, cache Cache, delegate blobstore.Blobs) blobstore.Blobs {
	return &CachedStatBlobstore{
		Blobs: delegate,
		cache: cache,
		log:   log,
	}
}

// BlobInfo is the cached version of blobstore.Blobinfo.
type BlobInfo struct {
	blobstore.BlobInfo
	cache Cache
}

// Stat implements blobstore.Blobinfo.
func (b BlobInfo) Stat(ctx context.Context) (blobstore.FileInfo, error) {
	ns := b.BlobInfo.BlobRef().Namespace
	key := b.BlobInfo.BlobRef().Key
	cached, found, err := b.cache.Get(ctx, ns, key)
	if found || err != nil {
		return cached, err
	}
	value, err := b.BlobInfo.Stat(ctx)
	if err != nil {
		return value, err
	}
	err = b.cache.Set(ctx, ns, key, value)
	if err != nil {
		return value, err
	}
	return value, err
}

// Stat implements blobstore.Blobs.
func (s *CachedStatBlobstore) Stat(ctx context.Context, ref blobstore.BlobRef) (blobstore.BlobInfo, error) {
	original, err := s.Blobs.Stat(ctx, ref)
	return BlobInfo{
		BlobInfo: original,
		cache:    s.cache,
	}, err
}

// StatWithStorageFormat implements blobstore.Blobs.
func (s *CachedStatBlobstore) StatWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (blobstore.BlobInfo, error) {
	original, err := s.Blobs.StatWithStorageFormat(ctx, ref, formatVer)
	return BlobInfo{
		BlobInfo: original,
		cache:    s.cache,
	}, err
}

// WalkNamespace implements blobstore.Blobs.
func (s *CachedStatBlobstore) WalkNamespace(ctx context.Context, namespace []byte, startFromPrefix string, walkFunc func(blobstore.BlobInfo) error) error {
	return s.Blobs.WalkNamespace(ctx, namespace, startFromPrefix, func(info blobstore.BlobInfo) error {
		return walkFunc(BlobInfo{
			BlobInfo: info,
			cache:    s.cache,
		})
	})
}

// Delete implements blobstore.Blobs.
func (s *CachedStatBlobstore) Delete(ctx context.Context, ref blobstore.BlobRef) error {
	err := s.cache.Delete(ctx, ref.Namespace, ref.Key)
	if err != nil {
		s.log.Warn("Couldn't delete blobstore cache entry", zap.Binary("namespace", ref.Namespace), zap.Binary("key", ref.Key), zap.Error(err))
	}
	return s.Blobs.Delete(ctx, ref)
}

// DeleteWithStorageFormat implements blobstore.Blobs.
func (s *CachedStatBlobstore) DeleteWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) error {
	err := s.cache.Delete(ctx, ref.Namespace, ref.Key)
	if err != nil {
		s.log.Warn("Couldn't delete blobstore cache entry", zap.Binary("namespace", ref.Namespace), zap.Binary("key", ref.Key), zap.Error(err))
	}
	return s.Blobs.DeleteWithStorageFormat(ctx, ref, formatVer)
}

// EmptyTrash implements blobstore.Blobs.
func (s *CachedStatBlobstore) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (int64, [][]byte, error) {
	size, trashed, err := s.Blobs.EmptyTrash(ctx, namespace, trashedBefore)
	if err != nil {
		return size, trashed, err
	}
	for _, k := range trashed {
		err := s.cache.Delete(ctx, namespace, k)
		if err != nil {
			s.log.Warn("Couldn't delete blobstore cache entry", zap.Binary("namespace", namespace), zap.Binary("key", k), zap.Error(err))
		}
	}
	return size, trashed, nil
}
