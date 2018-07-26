// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"io"
	"time"

	"github.com/golang/protobuf/ptypes"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/ec"
	opb "storj.io/storj/protos/overlay"
	ppb "storj.io/storj/protos/pointerdb"
)

var (
	mon = monkit.Package()
)

// Meta will contain encryption and stream information
type Meta struct {
	Data []byte
}

// Store for segments
type Store interface {
	Meta(ctx context.Context, path paths.Path) (meta Meta, err error)
	Get(ctx context.Context, path paths.Path) (rr ranger.RangeCloser,
		meta Meta, err error)
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
		expiration time.Time) (meta Meta, err error)
	Delete(ctx context.Context, path paths.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
		recursive bool, limit int, metaFlags uint64) (items []storage.ListItem,
		more bool, err error)
}

type segmentStore struct {
	oc  overlay.Client
	ec  ecclient.Client
	pdb pointerdb.Client
	rs  eestream.RedundancyStrategy
}

// NewSegmentStore creates a new instance of segmentStore
func NewSegmentStore(oc overlay.Client, ec ecclient.Client,
	pdb pointerdb.Client, rs eestream.RedundancyStrategy) Store {
	return &segmentStore{oc: oc, ec: ec, pdb: pdb, rs: rs}
}

// Meta retrieves the metadata of the segment
func (s *segmentStore) Meta(ctx context.Context, path paths.Path) (meta Meta,
	err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	return Meta{Data: pr.GetMetadata()}, nil
}

// Put uploads a segment to an erasure code client
func (s *segmentStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	// uses overlay client to request a list of nodes
	nodes, err := s.oc.Choose(ctx, s.rs.TotalCount(), 0)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	pieceID := client.NewPieceID()

	// puts file to ecclient
	err = s.ec.Put(ctx, nodes, s.rs, pieceID, data, expiration)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	var remotePieces []*ppb.RemotePiece
	for i := range nodes {
		remotePieces = append(remotePieces, &ppb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   nodes[i].Id,
		})
	}

	exp, err := ptypes.TimestampProto(expiration)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	// creates pointer
	pr := &ppb.Pointer{
		Type: ppb.Pointer_REMOTE,
		Remote: &ppb.RemoteSegment{
			Redundancy: &ppb.RedundancyScheme{
				Type:             ppb.RedundancyScheme_RS,
				MinReq:           int32(s.rs.RequiredCount()),
				Total:            int32(s.rs.TotalCount()),
				RepairThreshold:  int32(s.rs.Min),
				SuccessThreshold: int32(s.rs.Opt),
			},
			PieceId:      string(pieceID),
			RemotePieces: remotePieces,
		},
		ExpirationDate: exp,
		Metadata:       metadata,
	}

	// puts pointer to pointerDB
	err = s.pdb.Put(ctx, path, pr, nil)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	// get the metadata for the newly uploaded segment
	m, err := s.Meta(ctx, path)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}
	return m, nil
}

// Get retrieves a segment using erasure code, overlay, and pointerdb clients
func (s *segmentStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	if pr.GetType() != ppb.Pointer_REMOTE {
		return nil, Meta{}, Error.New("TODO: only getting remote pointers supported")
	}

	seg := pr.GetRemote()
	pid := client.PieceID(seg.PieceId)
	nodes, err := s.lookupNodes(ctx, seg)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	rr, err = s.ec.Get(ctx, nodes, s.rs, pid, pr.GetSize())
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	return rr, Meta{Data: pr.GetMetadata()}, nil
}

// Delete tells piece stores to delete a segment and deletes pointer from pointerdb
func (s *segmentStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	if pr.GetType() != ppb.Pointer_REMOTE {
		return Error.New("TODO: only getting remote pointers supported")
	}

	seg := pr.GetRemote()
	pid := client.PieceID(seg.PieceId)
	nodes, err := s.lookupNodes(ctx, seg)
	if err != nil {
		return Error.Wrap(err)
	}

	// ecclient sends delete request
	err = s.ec.Delete(ctx, nodes, pid)
	if err != nil {
		return Error.Wrap(err)
	}

	// deletes pointer from pointerdb
	return s.pdb.Delete(ctx, path, nil)
}

// lookupNodes calls Lookup to get node addresses from the overlay
func (s *segmentStore) lookupNodes(ctx context.Context, seg *ppb.RemoteSegment) (
	nodes []*opb.Node, err error) {
	nodes = make([]*opb.Node, len(seg.GetRemotePieces()))
	for i, p := range seg.GetRemotePieces() {
		node, err := s.oc.Lookup(ctx, kademlia.StringToNodeID(p.GetNodeId()))
		if err != nil {
			// TODO better error handling: failing to lookup a few nodes should
			// not fail the request
			return nil, Error.Wrap(err)
		}
		nodes[i] = node
	}
	return nodes, nil
}

// List retrieves paths to segments and their metadata stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint64) (
	items []storage.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.pdb.List(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags, nil)
}
