// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/storage/segments"
)

// Store interface methods for streams to satisfy to be a store
type Store interface {
	Meta(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) (Meta, error)
	Get(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) (ranger.Ranger, Meta, error)
	Put(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite, data io.Reader, metadata []byte, expiration time.Time) (Meta, error)
	Delete(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) error
	List(ctx context.Context, prefix, startAfter, endBefore storj.Path, pathCipher storj.CipherSuite, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

type shimStore struct {
	store typedStore
}

// NewStreamStore constructs a Store.
func NewStreamStore(segments segments.Store, segmentSize int64, encStore *encryption.Store, encBlockSize int, cipher storj.CipherSuite, inlineThreshold int) (Store, error) {
	typedStore, err := newTypedStreamStore(segments, segmentSize, encStore, encBlockSize, cipher, inlineThreshold)
	if err != nil {
		return nil, err
	}
	return &shimStore{store: typedStore}, nil
}

// Meta parses the passed in path and dispatches to the typed store.
func (s *shimStore) Meta(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) (_ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Meta(ctx, ParsePath(path), pathCipher)
}

// Get parses the passed in path and dispatches to the typed store.
func (s *shimStore) Get(ctx context.Context, path storj.Path, pathCipher storj.CipherSuite) (_ ranger.Ranger, _ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.Get(ctx, ParsePath(path), pathCipher)
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

// List parses the passed in path and dispatches to the typed store.
func (s *shimStore) List(ctx context.Context, prefix storj.Path, startAfter storj.Path, endBefore storj.Path, pathCipher storj.CipherSuite, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.List(ctx, ParsePath(prefix), startAfter, endBefore, pathCipher, recursive, limit, metaFlags)
}
