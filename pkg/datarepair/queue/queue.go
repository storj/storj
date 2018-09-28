// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"encoding/binary"
	"time"
	"sync"
	"github.com/golang/protobuf/proto"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/redis"
)

//RepairQueue is the interface for the data repair queue
type RepairQueue interface {
	Add(qi *pb.InjuredSegment) (storage.Key, error)
	Remove(qi *pb.InjuredSegment) error
	GetNext() (storage.Key, pb.InjuredSegment, error)
	GetSize() (int, error)
}

//Queue implements the RepairQueue interface
type Queue struct {
	DB storage.KeyValueStore
	mutex *sync.Mutex
}

var (
	queueError = errs.Class("data repair queue error")
)

//NewQueue returns a pointer to a new Queue instance with an initialized connection to Redis
func NewQueue(address, password string, db int) (*Queue, error) {
	rc, err := redis.NewClient(address, password, db)
	if err != nil {
		return nil, err
	}
	return &Queue{
		DB: rc,
		mutex: &sync.Mutex{},
	}, nil
}

//Add adds a repair segment to the queue
func (q *Queue) Add(qi *pb.InjuredSegment) (storage.Key, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	dateTime := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(dateTime, time.Now().UnixNano())
	val, err := proto.Marshal(qi)
	if err != nil {
		return nil, queueError.New("error marshalling injured seg %s", err)
	}
	err = q.DB.Put(dateTime, val)
	if err != nil {
		return nil, queueError.New("error adding injured seg to queue %s", err)
	}
	return dateTime, nil
}

//Remove removes a repair segment from the queue
func (q Queue) Remove(qi *pb.InjuredSegment) error {
	//TODO
	return nil
}

//GetNext returns the next repair segement from the queue
func (q Queue) GetNext() (storage.Key, pb.InjuredSegment, error) {
	//TODO
	return pb.InjuredSegment{}
}

//GetSize returns the number of repair segements are in the queue
func (q Queue) GetSize() int {
	//TODO
	return 0
}
