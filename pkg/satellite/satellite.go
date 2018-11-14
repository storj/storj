// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	pointerdbAuth "storj.io/storj/pkg/pointerdb/auth"
	"storj.io/storj/pkg/provider"
)

var (
	mon            = monkit.Package()
	satelliteError = errs.Class("satellite error")
)

type Satellite struct {
	logger   *zap.Logger
	pdb      *pointerdb.Server
	overlay  pb.OverlayServer
	identity *provider.FullIdentity

	pb.SatelliteServer
}

// NewSatelliteServer creates instance of
func NewSatelliteServer(overlay pb.OverlayServer, pdb *pointerdb.Server, logger *zap.Logger, identity *provider.FullIdentity) *Satellite {
	return &Satellite{
		logger:   logger,
		pdb:      pdb,
		overlay:  overlay,
		identity: identity,
	}
}

func (s *Satellite) PutInfo(ctx context.Context, req *pb.PutInfoRequest) (resp *pb.PutInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	// TODO(coyle): We will also need to communicate with the reputation service here
	response, err := s.overlay.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount:        int64(req.GetAmount()),
			Restrictions:  &pb.NodeRestrictions{FreeDisk: req.GetSpace()},
			ExcludedNodes: req.GetExcluded(),
		},
	})
	if err != nil {
		return nil, err
	}

	pba, err := s.getPayerBandwidthAllocation(ctx, pb.PayerBandwidthAllocation_PUT)
	if err != nil {
		s.logger.Error("err getting payer bandwidth allocation", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	authorization, err := s.getSignedMessage()
	if err != nil {
		s.logger.Error("err getting signed message", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &pb.PutInfoResponse{Nodes: response.GetNodes(), Pba: pba, Authorization: authorization}, nil
}

func (s *Satellite) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (resp *pb.GetInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	response, err := s.pdb.Get(ctx, &pb.GetRequest{Path: req.GetPath()})
	if err != nil {
		return nil, err
	}

	nodes := response.GetNodes()

	pba, err := s.getPayerBandwidthAllocation(ctx, pb.PayerBandwidthAllocation_GET)
	if err != nil {
		s.logger.Error("err getting payer bandwidth allocation", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	authorization, err := s.getSignedMessage()
	if err != nil {
		s.logger.Error("err getting signed message", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.GetInfoResponse{
		Pointer:       response.GetPointer(),
		Nodes:         nodes,
		Pba:           pba,
		Authorization: authorization,
	}, nil
}

func (s *Satellite) PutMeta(ctx context.Context, req *pb.PutRequest) (resp *pb.PutResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	return s.pdb.Put(ctx, req)
}

func (s *Satellite) DeleteMeta(ctx context.Context, req *pb.DeleteRequest) (resp *pb.DeleteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	return s.pdb.Delete(ctx, req)
}

func (s *Satellite) ListMeta(ctx context.Context, req *pb.ListRequest) (resp *pb.ListResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = s.validateAuth(ctx); err != nil {
		return nil, err
	}

	return s.pdb.List(ctx, req)
}

func (s *Satellite) validateAuth(ctx context.Context) error {
	APIKey, ok := auth.GetAPIKey(ctx)
	if !ok || !pointerdbAuth.ValidateAPIKey(string(APIKey)) {
		s.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

func (s *Satellite) getPayerBandwidthAllocation(ctx context.Context, action pb.PayerBandwidthAllocation_Action) (*pb.PayerBandwidthAllocation, error) {
	payer := s.identity.ID.Bytes()

	// TODO(michal) should be replaced with renter id when available
	peerIdentity, err := provider.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	pbad := &pb.PayerBandwidthAllocation_Data{
		SatelliteId:    payer,
		UplinkId:       peerIdentity.ID.Bytes(),
		CreatedUnixSec: time.Now().Unix(),
		Action:         action,
	}

	data, err := proto.Marshal(pbad)
	if err != nil {
		return nil, err
	}
	signature, err := auth.GenerateSignature(data, s.identity)
	if err != nil {
		return nil, err
	}
	return &pb.PayerBandwidthAllocation{Signature: signature, Data: data}, nil
}

func (s *Satellite) getSignedMessage() (*pb.SignedMessage, error) {
	signature, err := auth.GenerateSignature(s.identity.ID.Bytes(), s.identity)
	if err != nil {
		return nil, err
	}

	return auth.NewSignedMessage(signature, s.identity)
}

// lookupNodes calls Lookup to get node addresses from the overlay
func (s *Satellite) lookupNodes(ctx context.Context, seg *pb.RemoteSegment) (nodes []*pb.Node, err error) {
	// Get list of all nodes IDs storing a piece from the segment
	var requests []*pb.LookupRequest
	for _, p := range seg.RemotePieces {
		requests = append(requests, &pb.LookupRequest{NodeID: p.GetNodeId()})
	}
	// Lookup the node info from node IDs

	response, err := s.overlay.BulkLookup(ctx, &pb.LookupRequests{Lookuprequest: requests})
	if err != nil {
		return nil, satelliteError.Wrap(err)
	}
	n := response.GetLookupresponse()

	// Create an indexed list of nodes based on the piece number.
	// Missing pieces are represented by a nil node.
	nodes = make([]*pb.Node, seg.GetRedundancy().GetTotal())
	for i, p := range seg.GetRemotePieces() {
		nodes[p.PieceNum] = n[i].GetNode()
	}
	return nodes, nil
}
