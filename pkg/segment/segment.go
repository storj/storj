// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segment

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/protos/pointerdb"
	opb "storj.io/storj/protos/overlay"
)

// Meta describes associated Nodes and if data is Inline
type Meta struct {
	dtypes.Meta

	Inline bool
	Nodes  []overlay.NodeID
}

// Store allows Put, Get, Delete, and List methods on paths
type Store interface {
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
		expiration time.Time) error
	Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error)
	Delete(ctx context.Context, path paths.Path) error
	List(ctx context.Context, startingPath, endingPath paths.Path) (
		paths []paths.Path, truncated bool, err error)
}

// SegmentStore defines the SegmentStore
type segmentStore struct {
	pdb *pointerdb.PointerDBClient
	oc  *overlay.Overlay
	tc  *transport.Transport
}

// Put uploads a file to an erasure code client
func (s *SegmentStore) Put(ctx context.Context, path paths.Path, data io.Reader,
metadata []byte, expiration time.Time) err error {
	defer mon.Task()(&tcx)(&err)

	addr := "bootstrap.storj.io:7070"
	c, err := overlay.NewClient(&addr, grpc.WithInsecure())
	if err != nil {
		return Error.Wrap(err)
	}

	r, err := c.FindStorageNodes(ctx, &opb.FindStorageNodesRequest{})
	if err != nil {
		return Error.Wrap(err)
	}


}

// Get retrieves a file from the erasure code client with help from overlay and pointerdb
func (s *SegmentStore) Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error) {
}

// Delete issues deletes of a file to all piece stores and deletes from pointerdb
func (s *SegmentStore) Delete(ctx context.Context, path paths.Path) error {
}

// List lists paths stored in the pointerdb
func (s *SegmentStore) List(ctx context.Context, startingPath, endingPath paths.Path) (
paths []paths.Path, truncated bool, err error) {
}
