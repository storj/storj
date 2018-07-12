// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segment

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/transport"
	opb "storj.io/storj/protos/overlay"
	ppb "storj.io/storj/protos/pointerdb"
)

var (
	mon             = monkit.Package()
	netstateAddress = flag.String("netstate_addr", "35.202.58.176:4242", "address")
)

// Meta describes associated Nodes and if data is Inline or Remote
type Meta struct {
	Inline bool
	Nodes  []dht.NodeID
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
	oc opb.OverlayClient
	tc transport.Client
	// max buffer memory
	mbm int
	rs  eestream.RedundancyStrategy
}

// NewSegmentStore creates a new instance of segmentStore; mbm is max buffer memory
func NewSegmentStore(oc opb.OverlayClient, tc transport.Client,
	rs eestream.RedundancyStrategy, mbm int) Store {
	return &segmentStore{oc: oc, tc: tc, rs: rs, mbm: 2}
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
	ecc := ecclient.NewClient(s.tc, s.mbm)
	err = ecc.Put(ctx, nodeRes.GetNodes(), s.rs, pieceID, data, expiration)
	if err != nil {
		zap.S().Error("Failed putting nodes to ecclient")
		return Error.Wrap(err)
	}

	conn, err := grpc.Dial(*netstateAddress, grpc.WithInsecure())
	if err != nil {
		zap.S().Error("Failed to dial: ", zap.Error(err))
		return Error.Wrap(err)
	}
	pdc := ppb.NewPointerDBClient(conn)

	var remotePieces []*ppb.RemotePiece
	for i := range nodeRes.Nodes {
		remotePieces = append(remotePieces, &ppb.RemotePiece{
			PieceNum: int64(i),
			NodeId:   nodeRes.Nodes[i].Id,
		})
	}

	// creates pointer
	pr := ppb.PutRequest{
		Path: []byte(path),
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
				PieceId:      fmt.Sprintf("%s", pieceID),
				RemotePieces: remotePieces,
			},
		},
		APIKey: nil,
	}

	// puts pointer to pointerDB
	_, err = pdc.Put(ctx, &pr)
	if err != nil || status.Code(err) == codes.Internal {
		zap.L().Error("failed to put", zap.Error(err))
		return Error.Wrap(err)
	}
	return nil
}

// Get retrieves a file from the erasure code client with help from overlay and pointerdb
func (s *segmentStore) Get(ctx context.Context, path paths.Path) (ranger.Ranger, Meta, error) {
	m := Meta{
		Inline: true,
		Nodes:  nil,
	}
	return nil, m, nil
}

// Delete issues deletes of a file to all piece stores and deletes from pointerdb
func (s *segmentStore) Delete(ctx context.Context, path paths.Path) error {
	return nil
}

// List lists paths stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, startingPath, endingPath paths.Path) (
	paths []paths.Path, truncated bool, err error) {
	return nil, true, nil
}
