// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

type repairQueueDB struct {
	db *dbx.DB
}

func (r *repairQueueDB) Enqueue(ctx context.Context, seg storage.Value) error {
	_, err := r.db.Create_Injuredsegment(
		ctx,
		dbx.Injuredsegment_Info(seg),
	)
	return err
}

func (r *repairQueueDB) Dequeue(ctx context.Context) (storage.Value, error) {
	res, err := r.db.First_Injuredsegment(ctx)
	if err != nil {
		return nil, err
	}

	deleted, err := r.db.Delete_Injuredsegment_By_Info(
		ctx,
		dbx.Injuredsegment_Info(res.Info),
	)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, Error.New("Injured segment not deleted")
	}

	return res.Info, nil
}

func (r *repairQueueDB) Peekqueue(ctx context.Context, limit int) ([]storage.Value, error) {
	rows, err := r.db.Limited_Injuredsegment(ctx, limit, 0)
	if err != nil {
		return nil, err
	}

	segments := make([]storage.Value, len(rows))
	for i, entry := range rows {
		segments[i] = entry.Info
	}
	return segments, nil
}

func (r *repairQueueDB) Close() error {
	return errs.New("Not implemented")
}
