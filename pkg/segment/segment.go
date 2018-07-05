// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segment

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/dtypes"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/protos/netstate"
)

// Meta describes associated Nodes and if data is Inline
type Meta struct {
	dtypes.Meta

	Inline bool
	Nodes  []dtypes.NodeID
}

// Store allows Put, Get, Delete, and List methods on paths
type Store interface {
	Put(ctx context.Context, path dtypes.Path, data io.Reader, metadata []byte,
		expiration time.Time) error
	Get(ctx context.Context, path dtypes.Path) (ranger.Ranger, Meta, error)
	Delete(ctx context.Context, path dtypes.Path) error
	List(ctx context.Context, startingPath, endingPath dtypes.Path) (
		paths []dtypes.Path, truncated bool, err error)
}

// SegmentStore defines the SegmentStore
type segmentStore struct {
	nc *netstate.NetStateClient
	oc *overlay.Overlay
	tc *transport.Transport
}

// Put uploads a file to an erasure code client
func (s *SegmentStore) Put(ctx context.Context, path dtypes.Path, data io.Reader,
	metadata []byte, expiration time.Time) error {
}

// Get retrieves a file from the erasure code client with help from overlay and pointerdb
func (s *SegmentStore) Get(ctx context.Context, path dtypes.Path) (ranger.Ranger, Meta, error) {
}

// Delete issues deletes of a file to all piece stores and deletes from pointerdb
func (s *SegmentStore) Delete(ctx context.Context, path dtypes.Path) error {
}

// List lists paths stored in the pointerdb
func (s *SegmentStore) List(ctx context.Context, startingPath, endingPath dtypes.Path) (
	paths []dtypes.Path, truncated bool, err error) {
}
