// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segment

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/ec"
	opb "storj.io/storj/protos/overlay"
	pspb "storj.io/storj/protos/piecestore"
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
	oc  opb.OverlayClient
	ec  ecclient.Client
	ps  pspb.PieceStoreRoutesClient
	pdb ppb.PointerDBClient
	rs  eestream.RedundancyStrategy
}

// NewSegmentStore creates a new instance of segmentStore
func NewSegmentStore(oc opb.OverlayClient, ec ecclient.Client, ps pspb.PieceStoreRoutesClient,
	pdb ppb.PointerDBClient, rs eestream.RedundancyStrategy) Store {
	return &segmentStore{oc: oc, ec: ec, ps: ps, pdb: pdb, rs: rs}
}

// Put uploads a file to an erasure code client
func (s *segmentStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// uses overlay client to request a list of nodes
	nodeRes, err := s.oc.FindStorageNodes(ctx, &opb.FindStorageNodesRequest{})
	if err != nil {
		return Error.Wrap(err)
	}

	pieceID := client.NewPieceID()

	// puts file to ecclient
	err = s.ec.Put(ctx, nodeRes.GetNodes(), s.rs, pieceID, data, expiration)
	if err != nil {
		zap.S().Error("Failed putting nodes to ecclient")
		return Error.Wrap(err)
	}

	var remotePieces []*ppb.RemotePiece
	for i := range nodeRes.Nodes {
		remotePieces = append(remotePieces, &ppb.RemotePiece{
			PieceNum: int64(i),
			NodeId:   nodeRes.Nodes[i].Id,
		})
	}

	// creates pointer
	pr := ppb.PutRequest{
		Path: []byte(fmt.Sprintf("%s", path)),
		Pointer: &ppb.Pointer{
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
		},
		APIKey: nil,
	}

	// puts pointer to pointerDB
	_, err = s.pdb.Put(ctx, &pr)
	if err != nil || status.Code(err) == codes.Internal {
		zap.L().Error("failed to put", zap.Error(err))
		return Error.Wrap(err)
	}
	return nil
}

// Get retrieves a file using erasure code, overlay, and pointerdb clients
func (s *segmentStore) Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error) {
	m := Meta{}
	// TODO: remove this chunk after pointerdb client interface merged
	gr := &ppb.GetRequest{
		Path:   []byte(fmt.Sprintf("%s", path)),
		APIKey: nil,
	}

	pdbRes, err := s.pdb.Get(ctx, gr)
	if err != nil {
		return nil, m, err
	}

	// TODO: remove this chunk after pointerdb client interface merged
	pointer := &ppb.Pointer{}
	err = proto.Unmarshal(pdbRes.Pointer, pointer)
	if err != nil {
		return nil, m, err
	}

	if pointer.Type != ppb.Pointer_REMOTE {
		zap.L().Error("TODO: only getting remote pointers supported")
		return nil, m, err
	}

	remoteSeg := pointer.Remote
	var nodes []*opb.Node
	for i := 0; i < len(remoteSeg.RemotePieces); i++ {
		overlayRes, err := s.oc.Lookup(ctx, &opb.LookupRequest{NodeID: remoteSeg.RemotePieces[i].NodeId})
		if err != nil {
			return nil, m, err
		}
		nodes = append(nodes, overlayRes.Node)
	}
	pid := client.PieceID(remoteSeg.PieceId)
	ecRes, err := s.ec.Get(ctx, nodes, s.rs, pid, pointer.Size)
	if err != nil {
		return nil, m, err
	}

	m.Data = pointer.Metadata

	return ecRes, m, nil
}

// Delete tells piece stores to delete a segment and deletes pointer from pointerdb
func (s *segmentStore) Delete(ctx context.Context, path paths.Path) error {
	// TODO: remove this chunk after pointerdb client interface merged
	gr := &ppb.GetRequest{
		Path:   []byte(fmt.Sprintf("%s", path)),
		APIKey: nil,
	}

	// gets pointer from pointerdb
	pdbRes, err := s.pdb.Get(ctx, gr)
	if err != nil {
		return err
	}

	// TODO: remove this chunk after pointerdb client interface merged
	pointer := &ppb.Pointer{}
	err = proto.Unmarshal(pdbRes.Pointer, pointer)
	if err != nil {
		return err
	}

	// piece store client sends delete request
	_, err = s.ps.Delete(ctx, &pspb.PieceDelete{Id: pointer.Remote.PieceId})
	if err != nil {
		return err
	}

	// TODO: remove this chunk after pointerdb client interface merged
	dr := &ppb.DeleteRequest{
		Path:   []byte(fmt.Sprintf("%s", path)),
		APIKey: nil,
	}

	// deletes pointer from pointerdb
	_, err = s.pdb.Delete(ctx, dr)
	if err != nil {
		return err
	}

	return nil
}

// List lists paths stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, startingPath, endingPath paths.Path) (
	listPaths []paths.Path, truncated bool, err error) {

	// TODO: remove this chunk after pointerdb client interface merged
	lr := &ppb.ListRequest{
		StartingPathKey: []byte(fmt.Sprintf("%s", startingPath)),
		// TODO: change limit to endingPath when supported
		Limit:  1,
		APIKey: nil,
	}

	res, err := s.pdb.List(ctx, lr)
	if err != nil {
		return nil, false, err
	}

	for _, path := range res.Paths {
		var pathType []string
		pathType = append(pathType, string(path[:]))
		listPaths = append(listPaths, pathType)
	}

	return listPaths, res.Truncated, nil
}
