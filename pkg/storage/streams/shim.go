// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
)

// Store interface methods for streams to satisfy to be a store
type Store interface {
	Meta(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (Meta, error)
	Get(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (ranger.Ranger, Meta, error)
	Put(ctx context.Context, path storj.Path, pathCipher storj.Cipher, data io.Reader, metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path storj.Path, pathCipher storj.Cipher) error
	List(ctx context.Context, prefix, startAfter, endBefore storj.Path, pathCipher storj.Cipher, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

type shimStore struct {
	store typedStore
}

// NewStreamStore constructs a Store.
func NewStreamStore(segments segments.Store, segmentSize int64, rootKey *storj.Key, encBlockSize int, cipher storj.Cipher, inlineThreshold int) (Store, error) {
	typedStore, err := newTypedStreamStore(segments, segmentSize, rootKey, encBlockSize, cipher, inlineThreshold)
	if err != nil {
		return nil, err
	}
	return &shimStore{store: typedStore}, nil
}

func (s *shimStore) Meta(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (Meta, error) {
	return s.store.Meta(ctx, path, pathCipher)
}

func (s *shimStore) Get(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (ranger.Ranger, Meta, error) {
	return s.store.Get(ctx, path, pathCipher)
}

func (s *shimStore) Put(ctx context.Context, path storj.Path, pathCipher storj.Cipher, data io.Reader, metadata []byte, expiration time.Time) (Meta, error) {
	return s.store.Put(ctx, path, pathCipher, data, metadata, expiration)
}

func (s *shimStore) Delete(ctx context.Context, path storj.Path, pathCipher storj.Cipher) error {
	return s.store.Delete(ctx, path, pathCipher)
}

func (s *shimStore) List(ctx context.Context, prefix storj.Path, startAfter storj.Path, endBefore storj.Path, pathCipher storj.Cipher, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	return s.store.List(ctx, prefix, startAfter, endBefore, pathCipher, recursive, limit, metaFlags)
}
