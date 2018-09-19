// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

import "storj.io/storj/pkg/pb"

//Queue implements the RepairQueue interface
type Queue struct {

}

//NewQueue ..
func NewQueue() {

}

//Add ..
func (q Queue) Add(qi *pb.QueueItem) error {
	return nil
}

//AddAll ..
func (q Queue) AddAll(qis []*pb.QueueItem) error {
	return nil
}

//Remove ..
func (q Queue) Remove(qi *pb.QueueItem) error {
	return nil
}

//GetNext ..
func (q Queue) GetNext() pb.QueueItem {
	return pb.QueueItem{}
}

//GetAll ..
func (q Queue) GetAll() []*pb.QueueItem {
	return []*pb.QueueItem{}
}

//GetSize .. 
func (q Queue) GetSize() int {
	return 0
}
