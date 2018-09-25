// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import "storj.io/storj/pkg/pb"

//RepairQueue is the interface for the data repair queue
type RepairQueue interface {
	Add(qi *pb.InjuredSegment) error
	AddAll(qis []*pb.InjuredSegment) error
	Remove(qi *pb.InjuredSegment) error
	GetNext() pb.InjuredSegment
	GetSize() int
}

//Queue implements the RepairQueue interface
type Queue struct {
	//redis db of repair segments
	//offline nodes?
}

//NewQueue creates a new data repair queue
func NewQueue() RepairQueue {
	return &Queue{}
}

//Add adds a repair segment to the queue
func (q *Queue) Add(qi *pb.InjuredSegment) error {
	return nil
}

//AddAll adds a slice of repair segements to the queue
func (q *Queue) AddAll(qis []*pb.InjuredSegment) error {
	return nil
}

//Remove removes a repair segment from the queue
func (q *Queue) Remove(qi *pb.InjuredSegment) error {
	return nil
}

//GetNext returns the next repair segement from the queue
func (q *Queue) GetNext() pb.InjuredSegment {
	return pb.InjuredSegment{}
}

//GetSize returns the number of repair segements are in the queue
func (q *Queue) GetSize() int {
	return 0
}
