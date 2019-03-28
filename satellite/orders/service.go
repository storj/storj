// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type Service struct {
	satellite signing.Signer
	cache     *overlay.Cache
	certdb    certdb.DB

	orderExpiration time.Duration
}

func NewService() *Service { return &Service{} }

func (service *Service) CreateOrderLimits() ([]*pb.OrderLimit2, error) {
	return nil, nil
}

func (service *Service) createSerial(ctx context.Context, path storj.Path) (storj.SerialNumber, error) {
	// insert into table
	// associate with bucket
	return storj.SerialNumber{}, nil
}

func (service *Service) CreateAuditOrderLimits(ctx context.Context, auditor *identity.PeerIdentity, pointer *pb.Pointer) ([]*pb.AddressedOrderLimit, error) {
	rootPieceID := pointer.GetRemote().RootPieceId
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
	expiration := pointer.ExpirationDate

	// store the corresponding uplink's id and public key into certDB db
	err := service.certdb.SavePublicKey(ctx, auditor.ID, auditor.Leaf.PublicKey)
	if err != nil {
		return nil, err
	}

	bucketPath := storj.Path("TODO") // TODO:
	serialNumber, err := service.createSerial(ctx, bucketPath)
	if err != nil {
		return nil, err
	}

	// convert orderExpiration from days to timstamp
	orderExpiration, err := ptypes.TimestampProto(time.Now().Add(service.orderExpiration))
	if err != nil {
		return nil, err
	}

	limits := make([]*pb.AddressedOrderLimit, pointer.GetRemote().GetRedundancy().GetTotal())
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			// TODO: undo serial entry
			return nil, err
		}

		if node != nil {
			node.Type.DPanicOnInvalid("auditor order limits")
		}

		orderLimit, err := signing.SignOrderLimit(service.satellite, &pb.OrderLimit2{
			SerialNumber:    serialNumber,
			SatelliteId:     service.satellite.ID(),
			UplinkId:        auditor.ID,
			StorageNodeId:   piece.NodeId,
			PieceId:         rootPieceID.Derive(piece.NodeId),
			Action:          pb.PieceAction_GET_AUDIT,
			Limit:           int64(shareSize),
			PieceExpiration: expiration,
			OrderExpiration: orderExpiration,
		})

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	return limits, nil
}
