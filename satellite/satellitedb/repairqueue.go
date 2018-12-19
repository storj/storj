// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/golang/protobuf/proto"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type repairQueue struct {
	db *dbx.DB
}

func (r *repairQueue) Enqueue(ctx context.Context, seg *pb.InjuredSegment) error {
	val, err := proto.Marshal(seg)
	if err != nil {
		return err
	}

	_, err = r.db.Create_Injuredsegment(
		ctx,
		dbx.Injuredsegment_Info(val),
	)
	return err
}

func (r *repairQueue) Dequeue(ctx context.Context) (pb.InjuredSegment, error) {
	tx, err := r.db.Open(ctx)
	if err != nil {
		return pb.InjuredSegment{}, Error.Wrap(err)
	}

	res, err := tx.First_Injuredsegment(ctx)
	if err != nil {
		return pb.InjuredSegment{}, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}
	if res == nil {
		return pb.InjuredSegment{}, Error.Wrap(utils.CombineErrors(Error.New("Empty queue"), tx.Rollback()))
	}

	deleted, err := tx.Delete_Injuredsegment_By_Info(
		ctx,
		dbx.Injuredsegment_Info(res.Info),
	)
	if err != nil {
		return pb.InjuredSegment{}, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}
	if !deleted {
		return pb.InjuredSegment{}, Error.Wrap(utils.CombineErrors(Error.New("Injured segment not deleted"), tx.Rollback()))
	}

	seg := &pb.InjuredSegment{}
	err = proto.Unmarshal(res.Info, seg)
	if err != nil {
		return pb.InjuredSegment{}, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}
	return *seg, Error.Wrap(tx.Commit())
}

func (r *repairQueue) Peekqueue(ctx context.Context, limit int) ([]pb.InjuredSegment, error) {
	rows, err := r.db.Limited_Injuredsegment(ctx, limit, 0)
	if err != nil {
		return nil, err
	}

	segments := make([]pb.InjuredSegment, 0)
	for _, entry := range rows {
		seg := &pb.InjuredSegment{}
		if err = proto.Unmarshal(entry.Info, seg); err != nil {
			return nil, err
		}
		segments = append(segments, *seg)
	}
	return segments, nil
}
