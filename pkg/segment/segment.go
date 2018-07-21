// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segment

import (
	"context"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/ranger"
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

// Store allows Put, Get, Delete, and List methods on paths
type Store interface {
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
		expiration time.Time) error
	Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error)
	Delete(ctx context.Context, path paths.Path) error
	List(ctx context.Context, startingPath, endingPath paths.Path) (
		paths []paths.Path, truncated bool, err error)
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

// Put uploads a file to an erasure code client
func (s *segmentStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// uses overlay client to request a list of nodes
	nodeRes, err := s.oc.Choose(ctx, 0, 0)
	if err != nil {
		return Error.Wrap(err)
	}

	pieceID := client.NewPieceID()

	// puts file to ecclient
	err = s.ec.Put(ctx, nodeRes, s.rs, pieceID, data, expiration)
	if err != nil {
		zap.S().Error("Failed putting nodes to ecclient")
		return Error.Wrap(err)
	}

	var remotePieces []*ppb.RemotePiece
	for i := range nodeRes {
		remotePieces = append(remotePieces, &ppb.RemotePiece{
			PieceNum: int64(i),
			NodeId:   nodeRes[i].Id,
		})
	}

	// creates pointer
	pr := &ppb.Pointer{
		Type: ppb.Pointer_REMOTE,
		Remote: &ppb.RemoteSegment{
			Redundancy: &ppb.RedundancyScheme{
				Type:             ppb.RedundancyScheme_RS,
				MinReq:           int64(s.rs.RequiredCount()),
				Total:            int64(s.rs.TotalCount()),
				RepairThreshold:  int64(s.rs.Min),
				SuccessThreshold: int64(s.rs.Opt),
			},
			PieceId:      string(pieceID),
			RemotePieces: remotePieces,
		},
		Metadata: metadata,
	}

	// puts pointer to pointerDB
	err = s.pdb.Put(ctx, path, pr, nil)
	if err != nil || status.Code(err) == codes.Internal {
		zap.L().Error("failed to put", zap.Error(err))
		return Error.Wrap(err)
	}
	return nil
}

// Get retrieves a file using erasure code, overlay, and pointerdb clients
func (s *segmentStore) Get(ctx context.Context, path paths.Path) (
	ranger.Ranger, Meta, error) {
	m := Meta{}

	pointer, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return nil, m, err
	}

	if pointer.Type != ppb.Pointer_REMOTE {
		zap.L().Error("TODO: only getting remote pointers supported")
		return nil, m, err
	}

	nodes, err := s.overlayHelper(ctx, pointer.Remote)
	if err != nil {
		return nil, m, err
	}

	pid := client.PieceID(pointer.Remote.PieceId)
	ecRes, err := s.ec.Get(ctx, nodes, s.rs, pid, pointer.Size)
	if err != nil {
		return nil, m, err
	}

	m.Data = pointer.Metadata

	return ecRes, m, nil
}

// Delete tells piece stores to delete a segment and deletes pointer from
// pointerdb
func (s *segmentStore) Delete(ctx context.Context, path paths.Path) error {
	// gets pointer from pointerdb
	pointer, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return err
	}

	nodes, err := s.overlayHelper(ctx, pointer.Remote)
	if err != nil {
		return err
	}

	// ecclient sends delete request
	err = s.ec.Delete(ctx, nodes, client.PieceID(pointer.Remote.PieceId))
	if err != nil {
		return err
	}

	// deletes pointer from pointerdb
	err = s.pdb.Delete(ctx, path, nil)
	if err != nil {
		return err
	}

	return nil
}

// overlayHelper calls Lookup to get node addresses from the overlay
func (s *segmentStore) overlayHelper(ctx context.Context,
	rem *ppb.RemoteSegment) (nodes []*opb.Node, err error) {
	for i := 0; i < len(rem.RemotePieces); i++ {
		overlayRes, err := s.oc.Lookup(ctx,
			kademlia.StringToNodeID(rem.RemotePieces[i].NodeId))
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, overlayRes)
	}
	return nodes, nil
}

// List lists paths stored in the pointerdb
func (s *segmentStore) List(ctx context.Context,
	startingPath, endingPath paths.Path) (
	listPaths []paths.Path, truncated bool, err error) {

	pathsResp, truncated, err := s.pdb.List(
		ctx, startingPath, 0, nil)
	if err != nil {
		return nil, false, err
	}

	for _, path := range pathsResp {
		np := paths.New(string(path[:]))
		listPaths = append(listPaths, np)
	}

	return listPaths, truncated, nil
}
