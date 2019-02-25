// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

// RepairQueue implements queueing for segments that need repairing.
type RepairQueue interface {
	// Enqueue adds an injured segment.
	Enqueue(ctx context.Context, qi *pb.InjuredSegment) error
	// Dequeue removes an injured segment.
	Dequeue(ctx context.Context) (pb.InjuredSegment, error)
	// Peekqueue lists limit amount of injured segments.
	Peekqueue(ctx context.Context, limit int) ([]pb.InjuredSegment, error)
}

// Queue implements the RepairQueue interface
type Queue struct {
	db storage.Queue
}

// NewQueue returns a pointer to a new Queue instance with an initialized connection to Redis
func NewQueue(client storage.Queue) *Queue {
	zap.L().Info("Initializing new data repair queue")
	return &Queue{db: client}
}

// Enqueue adds a repair segment to the queue
func (q *Queue) Enqueue(ctx context.Context, qi *pb.InjuredSegment) error {
	val, err := proto.Marshal(qi)
	if err != nil {
		return Error.New("error marshalling injured seg %s", err)
	}

	err = q.db.Enqueue(val)
	if err != nil {
		return Error.New("error adding injured seg to queue %s", err)
	}
	return nil
}

// Dequeue returns the next repair segement and removes it from the queue
func (q *Queue) Dequeue(ctx context.Context) (pb.InjuredSegment, error) {
	val, err := q.db.Dequeue()
	if err != nil {
		if storage.ErrEmptyQueue.Has(err) {
			return pb.InjuredSegment{}, err
		}
		return pb.InjuredSegment{}, Error.New("error obtaining item from repair queue %s", err)
	}
	seg := &pb.InjuredSegment{}
	err = proto.Unmarshal(val, seg)
	if err != nil {
		return pb.InjuredSegment{}, Error.New("error unmarshalling segment %s", err)
	}
	return *seg, nil
}

// Peekqueue returns upto 'limit' of the entries from the repair queue
func (q *Queue) Peekqueue(ctx context.Context, limit int) ([]pb.InjuredSegment, error) {
	if limit < 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}
	result, err := q.db.Peekqueue(limit)
	if err != nil {
		return []pb.InjuredSegment{}, Error.New("error peeking into repair queue %s", err)
	}
	segs := make([]pb.InjuredSegment, 0)
	for _, v := range result {
		seg := &pb.InjuredSegment{}
		if err = proto.Unmarshal(v, seg); err != nil {
			return nil, err
		}
		segs = append(segs, *seg)
	}
	return segs, nil
}
