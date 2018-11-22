// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package distributor

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
	satelliteError = errs.Class("distributor error")
)

type Distributor struct {
	logger   *zap.Logger
	pdb      *pointerdb.Server
	overlay  pb.OverlayServer
	identity *provider.FullIdentity
}

// NewSatelliteServer creates instance of
func NewDistributorServer(overlay pb.OverlayServer, pdb *pointerdb.Server, logger *zap.Logger, identity *provider.FullIdentity) *Distributor {
	return &Distributor{
		logger:   logger,
		pdb:      pdb,
		overlay:  overlay,
		identity: identity,
	}
}

func (d *Distributor) PutInfo(ctx context.Context, req *pb.PutInfoRequest) (resp *pb.PutInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = d.validateAuth(ctx); err != nil {
		return nil, err
	}

	// TODO(coyle): We will also need to communicate with the reputation service here
	response, err := d.overlay.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount:        int64(req.GetAmount()),
			Restrictions:  &pb.NodeRestrictions{FreeDisk: req.GetSpace()},
			ExcludedNodes: req.GetExcluded(),
		},
	})
	if err != nil {
		return nil, err
	}

	pba, err := d.getPayerBandwidthAllocation(ctx, pb.PayerBandwidthAllocation_PUT)
	if err != nil {
		d.logger.Error("err getting payer bandwidth allocation", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	authorization, err := d.getSignedMessage()
	if err != nil {
		d.logger.Error("err getting signed message", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &pb.PutInfoResponse{Nodes: response.GetNodes(), Pba: pba, Authorization: authorization}, nil
}

func (d *Distributor) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (resp *pb.GetInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = d.validateAuth(ctx); err != nil {
		return nil, err
	}

	response, err := d.pdb.Get(ctx, &pb.GetRequest{Path: req.GetPath()})
	if err != nil {
		return nil, err
	}

	nodes := response.GetNodes()

	pba, err := d.getPayerBandwidthAllocation(ctx, pb.PayerBandwidthAllocation_GET)
	if err != nil {
		d.logger.Error("err getting payer bandwidth allocation", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	authorization, err := d.getSignedMessage()
	if err != nil {
		d.logger.Error("err getting signed message", zap.Error(err))
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.GetInfoResponse{
		Pointer:       response.GetPointer(),
		Nodes:         nodes,
		Pba:           pba,
		Authorization: authorization,
	}, nil
}

func (d *Distributor) validateAuth(ctx context.Context) error {
	APIKey, ok := auth.GetAPIKey(ctx)
	if !ok || !pointerdbAuth.ValidateAPIKey(string(APIKey)) {
		d.logger.Error("unauthorized request: ", zap.Error(status.Errorf(codes.Unauthenticated, "Invalid API credential")))
		return status.Errorf(codes.Unauthenticated, "Invalid API credential")
	}
	return nil
}

func (d *Distributor) getPayerBandwidthAllocation(ctx context.Context, action pb.PayerBandwidthAllocation_Action) (*pb.PayerBandwidthAllocation, error) {
	payer := d.identity.ID.Bytes()
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
	signature, err := auth.GenerateSignature(data, d.identity)
	if err != nil {
		return nil, err
	}
	return &pb.PayerBandwidthAllocation{Signature: signature, Data: data}, nil
}

func (d *Distributor) getSignedMessage() (*pb.SignedMessage, error) {
	signature, err := auth.GenerateSignature(d.identity.ID.Bytes(), d.identity)
	if err != nil {
		return nil, err
	}
	return auth.NewSignedMessage(signature, d.identity)
}
