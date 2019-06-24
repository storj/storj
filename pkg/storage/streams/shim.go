// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
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
func NewStreamStore(segments segments.Store, segmentSize int64, encStore *encryption.Store, encBlockSize int, cipher storj.Cipher, inlineThreshold int) (Store, error) {
	typedStore, err := newTypedStreamStore(segments, segmentSize, encStore, encBlockSize, cipher, inlineThreshold)
	if err != nil {
		return nil, err
	}
	return &shimStore{store: typedStore}, nil
}

// Meta parses the passed in path and dispatches to the typed store.
func (s *shimStore) Meta(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (_ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	streamsPath, err := ParsePath(ctx, []byte(path))
	if err != nil {
		return Meta{}, err
	}
	return s.store.Meta(ctx, streamsPath, pathCipher)
}

// Get parses the passed in path and dispatches to the typed store.
func (s *shimStore) Get(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (_ ranger.Ranger, _ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	streamsPath, err := ParsePath(ctx, []byte(path))
	if err != nil {
		return nil, Meta{}, err
	}
	return s.store.Get(ctx, streamsPath, pathCipher)
}

// Put parses the passed in path and dispatches to the typed store.
func (s *shimStore) Put(ctx context.Context, path storj.Path, pathCipher storj.Cipher, data io.Reader, metadata []byte, expiration time.Time) (_ Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	streamsPath, err := ParsePath(ctx, []byte(path))
	if err != nil {
		return Meta{}, err
	}
	return s.store.Put(ctx, streamsPath, pathCipher, data, metadata, expiration)
}

// Delete parses the passed in path and dispatches to the typed store.
func (s *shimStore) Delete(ctx context.Context, path storj.Path, pathCipher storj.Cipher) (err error) {
	defer mon.Task()(&ctx)(&err)

	streamsPath, err := ParsePath(ctx, []byte(path))
	if err != nil {
		return err
	}
	return s.store.Delete(ctx, streamsPath, pathCipher)
}

// List parses the passed in path and dispatches to the typed store.
func (s *shimStore) List(ctx context.Context, prefix storj.Path, startAfter storj.Path, endBefore storj.Path, pathCipher storj.Cipher, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: list is maybe wrong?
	streamsPrefix, err := ParsePath(ctx, []byte(prefix))
	if err != nil {
		return nil, false, err
	}
	return s.store.List(ctx, streamsPrefix, startAfter, endBefore, pathCipher, recursive, limit, metaFlags)
}

// EncryptAfterBucket encrypts a path without encrypting its first element. This is a legacy function
// that should no longer be needed after the typed path refactoring.
func EncryptAfterBucket(ctx context.Context, path storj.Path, cipher storj.Cipher, key *storj.Key) (encrypted storj.Path, err error) {
	defer mon.Task()(&ctx)(&err)

	comps := storj.SplitPath(path)
	if len(comps) <= 1 {
		return path, nil
	}

	encrypted, err = encryption.EncryptPath(path, cipher, key)
	if err != nil {
		return "", err
	}

	// replace the first path component with the unencrypted bucket name
	return storj.JoinPaths(comps[0], storj.JoinPaths(storj.SplitPath(encrypted)[1:]...)), nil
}

// DecryptStreamInfo decrypts stream info. This is a legacy function that should no longer
// be needed after the typed path refactoring.
func DecryptStreamInfo(ctx context.Context, streamMetaBytes []byte, path storj.Path, rootKey *storj.Key) (
	streamInfo []byte, streamMeta pb.StreamMeta, err error) {
	defer mon.Task()(&ctx)(&err)

	streamsPath, err := ParsePath(ctx, []byte(path))
	if err != nil {
		return nil, pb.StreamMeta{}, err
	}

	store := encryption.NewStore()
	store.SetDefaultKey(rootKey)

	return TypedDecryptStreamInfo(ctx, streamMetaBytes, streamsPath, store)
}
