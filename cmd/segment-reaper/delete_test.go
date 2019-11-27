// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

func TestDeleteSegment(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db := teststore.New()
	defer ctx.Check(db.Close)

	{
		err := makeSegment(ctx, db, "path1", time.Unix(10, 0))
		require.NoError(t, err)

		dryRun := false
		deleteError := deleteSegment(ctx, db, "path1", time.Unix(10, 0), dryRun)
		require.NoError(t, deleteError)
		_, err = db.Get(ctx, storage.Key("path1"))
		require.Error(t, err) // segment is deleted
	}
	{
		err := makeSegment(ctx, db, "path2", time.Unix(10, 0))
		require.NoError(t, err)

		dryRun := true
		deleteError := deleteSegment(ctx, db, "path2", time.Unix(10, 0), dryRun)
		require.NoError(t, deleteError)
		_, err = db.Get(ctx, storage.Key("path2"))
		require.NoError(t, err) // segment is not deleted because of dryRun
	}
	{
		err := makeSegment(ctx, db, "path3", time.Unix(10, 0))
		require.NoError(t, err)

		dryRun := false
		deleteError := deleteSegment(ctx, db, "path3", time.Unix(99, 0), dryRun)
		require.Error(t, deleteError)
		_, err = db.Get(ctx, storage.Key("path3"))
		require.NoError(t, err) // segment is not deleted because of time mismatch
	}
	{
		dryRun := false
		deleteError := deleteSegment(ctx, db, "not-existing-path", time.Unix(10, 0), dryRun)
		require.Error(t, deleteError)
	}
}

func makeSegment(ctx context.Context, db metainfo.PointerDB, path string, creationDate time.Time) error {
	pointer := &pb.Pointer{
		CreationDate: creationDate,
	}

	pointerBytes, err := proto.Marshal(pointer)
	if err != nil {
		return err
	}

	err = db.Put(ctx, storage.Key(path), storage.Value(pointerBytes))
	if err != nil {
		return err
	}

	return nil
}
