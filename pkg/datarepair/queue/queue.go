// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"github.com/golang/protobuf/proto"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

// RepairQueue is the interface for the data repair queue
type RepairQueue interface {
	Enqueue(qi *pb.InjuredSegment) error
	Dequeue() (pb.InjuredSegment, error)
}

// Queue implements the RepairQueue interface
type Queue struct {
	db storage.Queue
}

// NewQueue returns a pointer to a new Queue instance with an initialized connection to Redis
func NewQueue(client storage.Queue) *Queue {
	return &Queue{db: client}
}

// Enqueue adds a repair segment to the queue
func (q *Queue) Enqueue(qi *pb.InjuredSegment) error {
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
func (q *Queue) Dequeue() (pb.InjuredSegment, error) {
	val, err := q.db.Dequeue()
	if err != nil {
		return pb.InjuredSegment{}, Error.New("error obtaining item from repair queue %s", err)
	}
	seg := &pb.InjuredSegment{}
	err = proto.Unmarshal(val, seg)
	if err != nil {
		return pb.InjuredSegment{}, Error.New("error unmarshalling segment %s", err)
	}
	return *seg, nil
}
