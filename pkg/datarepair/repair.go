// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

import (
	"storj.io/storj/pkg/pb"
)

//RepairQueue is the interface for the data repair queue
type RepairQueue interface {
	Add(qi *pb.InjuredSegment) error
	AddAll(qis []*pb.InjuredSegment) error
	Remove(qi *pb.InjuredSegment) error
	GetNext() pb.InjuredSegment
	GetSize() int
}
