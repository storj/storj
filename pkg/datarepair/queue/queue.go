// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

//RepairQueue is the interface for the data repair queue
type RepairQueue interface {
	Add(qi *pb.InjuredSegment) (storage.Key, error)
	Remove(date storage.Key) error
	GetNext() (storage.Key, pb.InjuredSegment, error)
}

//Queue implements the RepairQueue interface
type Queue struct {
	mu sync.Mutex
	db storage.KeyValueStore
}

var (
	queueError = errs.Class("data repair queue error")
)

//NewQueue returns a pointer to a new Queue instance with an initialized connection to Redis
func NewQueue(client storage.KeyValueStore) *Queue {
	return &Queue{
		mu: sync.Mutex{},
		db:    client,
	}
}

//Add adds a repair segment to the queue
func (q *Queue) Add(qi *pb.InjuredSegment) (storage.Key, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	dateTime := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(dateTime, time.Now().UnixNano())
	val, err := proto.Marshal(qi)
	if err != nil {
		return nil, queueError.New("error marshalling injured seg %s", err)
	}
	err = q.db.Put(dateTime, val)
	if err != nil {
		return nil, queueError.New("error adding injured seg to queue %s", err)
	}
	return dateTime, nil
}

//Remove removes a repair segment from the queue
func (q Queue) Remove(date storage.Key) error {
	//TODO
	return nil
}

//GetNext returns the next repair segement from the queue
func (q Queue) GetNext() (storage.Key, pb.InjuredSegment, error) {
	//TODO
	return storage.Key{}, pb.InjuredSegment{}, nil
}

