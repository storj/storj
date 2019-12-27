// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

func TestDeleteSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db := teststore.New()
	defer ctx.Check(db.Close)

	t.Run("segment is deleted", func(t *testing.T) {
		_, err := makeSegment(ctx, db, "path1", time.Unix(10, 0))
		require.NoError(t, err)

		dryRun := false
		deleteError := deleteSegment(ctx, db, "path1", time.Unix(10, 0), dryRun)
		require.NoError(t, deleteError)
		_, err = db.Get(ctx, storage.Key("path1"))
		require.Error(t, err)
		require.True(t, storage.ErrKeyNotFound.Has(err))
	})
	t.Run("segment is not deleted because of dryRun", func(t *testing.T) {
		expectedPointer, err := makeSegment(ctx, db, "path2", time.Unix(10, 0))
		require.NoError(t, err)

		dryRun := true
		deleteError := deleteSegment(ctx, db, "path2", time.Unix(10, 0), dryRun)
		require.NoError(t, deleteError)
		pointer, err := db.Get(ctx, storage.Key("path2"))
		require.NoError(t, err)
		pointerBytes, err := pointer.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, expectedPointer, pointerBytes)
	})
	t.Run("segment is not deleted because of time mismatch", func(t *testing.T) {
		expectedPointer, err := makeSegment(ctx, db, "path3", time.Unix(10, 0))
		require.NoError(t, err)

		dryRun := false
		deleteError := deleteSegment(ctx, db, "path3", time.Unix(99, 0), dryRun)
		require.Error(t, deleteError)
		require.True(t, errKnown.Has(deleteError))
		pointer, err := db.Get(ctx, storage.Key("path3"))
		require.NoError(t, err)
		pointerBytes, err := pointer.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, expectedPointer, pointerBytes)
	})
	t.Run("segment is not deleted because not exists", func(t *testing.T) {
		dryRun := false
		deleteError := deleteSegment(ctx, db, "not-existing-path", time.Unix(10, 0), dryRun)
		require.Error(t, deleteError)
		require.True(t, errKnown.Has(deleteError))
	})
}

func makeSegment(ctx context.Context, db metainfo.PointerDB, path string, creationDate time.Time) (pointerBytes []byte, err error) {
	pointer := &pb.Pointer{
		CreationDate: creationDate,
	}

	pointerBytes, err = proto.Marshal(pointer)
	if err != nil {
		return []byte{}, err
	}

	err = db.Put(ctx, storage.Key(path), storage.Value(pointerBytes))
	if err != nil {
		return []byte{}, err
	}

	return pointerBytes, nil
}
