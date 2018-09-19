// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

import (
	"storj.io/storj/pkg/pb"
)

//RepairQueue is the interface for the data repair queue
type RepairQueue interface {
	Add(qi *pb.QueueItem) error
	AddAll(qis []*pb.QueueItem) error
	Remove(qi *pb.QueueItem) error
	GetNext() pb.QueueItem
	GetSize() int
}
