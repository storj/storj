// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
)

// DeleteRequest contains info needed for a storage node to delete garbage data
type DeleteRequest struct {
	Node      storj.NodeID
	Filter    *bloomfilter.Filter
	Timestamp time.Time
}

// Queue defines the functions of the satellite's garbage collection queue
// TODO: does this need to be an interface?
type Queue interface {
	// Add adds a DeleteRequest to the Queue
	Add(ctx context.Context, nodeID storj.NodeID, filter *bloomfilter.Filter) error
}

type queue struct {
	Requests []*DeleteRequest
}

// Add adds a DeleteRequest to the Queue
func Add(ctx context.Context, nodeID storj.NodeID, filter *bloomfilter.Filter) (err error) {
	defer mon.Task()(&ctx)(&err)

	_ = &DeleteRequest{
		Node:      nodeID,
		Filter:    filter,
		Timestamp: time.Now().UTC(),
	}

	return nil
}
