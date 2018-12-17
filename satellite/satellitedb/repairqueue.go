// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/golang/protobuf/proto"
	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type repairQueueDB struct {
	db *dbx.DB
}

func (r *repairQueueDB) Enqueue(ctx context.Context, seg *pb.InjuredSegment) error {
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

func (r *repairQueueDB) Dequeue(ctx context.Context) (pb.InjuredSegment, error) {
	res, err := r.db.First_Injuredsegment(ctx)
	if err != nil {
		return pb.InjuredSegment{}, err
	}
	if res == nil {
		return pb.InjuredSegment{}, Error.New("Empty queue")
	}

	deleted, err := r.db.Delete_Injuredsegment_By_Info(
		ctx,
		dbx.Injuredsegment_Info(res.Info),
	)
	if err != nil {
		return pb.InjuredSegment{}, err
	}
	if !deleted {
		return pb.InjuredSegment{}, Error.New("Injured segment not deleted")
	}

	seg := &pb.InjuredSegment{}
	err = proto.Unmarshal(res.Info, seg)
	if err != nil {
		return pb.InjuredSegment{}, err
	}
	return *seg, nil
}

func (r *repairQueueDB) Peekqueue(ctx context.Context, limit int) ([]pb.InjuredSegment, error) {
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
