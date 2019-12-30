// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/common/encryption"
	"storj.io/common/ranger"
	"storj.io/common/storj"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/segments"
)

// Store interface methods for streams to satisfy to be a store
type Store interface {
	Get(ctx context.Context, path storj.Path, object storj.Object, pathCipher storj.CipherSuite) (ranger.Ranger, error)
	Put(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite, data io.Reader, metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) error
}

type shimStore struct {
	store typedStore
}

// NewStreamStore constructs a Store.
func NewStreamStore(metainfo *metainfo.Client, segments segments.Store, segmentSize int64, encStore *encryption.Store, encBlockSize int, cipher storj.CipherSuite, inlineThreshold int, maxEncryptedSegmentSize int64) (Store, error) {
	typedStore, err := newTypedStreamStore(metainfo, segments, segmentSize, encStore, encBlockSize, cipher, inlineThreshold, maxEncryptedSegmentSize)
	if err != nil {
		return nil, err
	}
	return &shimStore{store: typedStore}, nil
}

// Get parses the passed in path and dispatches to the typed store.
func (s *shimStore) Get(ctx context.Context, path storj.Path, object storj.Object, pathCipher storj.CipherSuite) (_ ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Get(ctx, ParsePath(path), object, pathCipher)
}

// Put parses the passed in path and dispatches to the typed store.
func (s *shimStore) Put(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite, data io.Reader, metadata []byte, expiration time.Time) (_ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Put(ctx, ParsePath(path), pathCipher, data, metadata, expiration)
}

// Delete parses the passed in path and dispatches to the typed store.
func (s *shimStore) Delete(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) (err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Delete(ctx, ParsePath(path), pathCipher)
}
