// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

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

// VerifyOrderLimitSignature verifies that the signature inside order limit belongs to the satellite.
func (service *Service) VerifyOrderLimitSignature(signed *pb.OrderLimit2) error {
	return signing.VerifyOrderLimitSignature(service.satellite, signed)
}

func (service *Service) createSerial(ctx context.Context, bucketPath storj.Path) (storj.SerialNumber, error) {
	// TODO
	return storj.SerialNumber{}, nil
}

func (service *Service) saveSerial(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte) error {
	return nil
}

func (service *Service) CreatePutOrderLimits(ctx context.Context, uplink *identity.PeerIdentity, nodes []*pb.Node, expiration *timestamp.Timestamp, maxPieceSize int64) (storj.PieceID, []*pb.AddressedOrderLimit, error) {
	bucketPath := storj.Path("TODO") // TODO:
	serialNumber, err := service.createSerial(ctx, bucketPath)
	if err != nil {
		return storj.PieceID{}, nil, err
	}
	// defer service.saveSerial(ctx, serialNumber, ...)

	// convert orderExpiration from days to timstamp
	orderExpiration, err := ptypes.TimestampProto(time.Now().Add(service.orderExpiration))
	if err != nil {
		return storj.PieceID{}, nil, Error.Wrap(err)
	}

	rootPieceID := storj.NewPieceID()
	limits := make([]*pb.AddressedOrderLimit, len(nodes))
	var pieceNum int32
	for _, node := range nodes {
		orderLimit, err := signing.SignOrderLimit(service.satellite, &pb.OrderLimit2{
			SerialNumber:    serialNumber,
			SatelliteId:     service.satellite.ID(),
			UplinkId:        uplink.ID,
			StorageNodeId:   node.Id,
			PieceId:         rootPieceID.Derive(node.Id),
			Action:          pb.PieceAction_PUT,
			Limit:           maxPieceSize,
			PieceExpiration: expiration,
			OrderExpiration: orderExpiration,
		})
		if err != nil {
			return storj.PieceID{}, nil, Error.Wrap(err)
		}

		limits[pieceNum] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		pieceNum++
	}

	return rootPieceID, limits, nil
}

func (service *Service) CreateAuditOrderLimits(ctx context.Context, auditor *identity.PeerIdentity, pointer *pb.Pointer) ([]*pb.AddressedOrderLimit, error) {
	rootPieceID := pointer.GetRemote().RootPieceId
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
	totalPieces := pointer.GetRemote().GetRedundancy().GetTotal()
	expiration := pointer.ExpirationDate

	bucketPath := storj.Path("TODO") // TODO:
	serialNumber, err := service.createSerial(ctx, bucketPath)
	if err != nil {
		return nil, err
	}
	// defer service.saveSerial(ctx, serialNumber, ...)

	// convert orderExpiration from days to timstamp
	orderExpiration, err := ptypes.TimestampProto(time.Now().Add(service.orderExpiration))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			// TODO: audit should not fail if a single node cannot be retrieved from overlay cache or is offline
			// TODO: undo serial entry
			return nil, Error.Wrap(err)
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
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	return limits, nil
}

func (service *Service) CreateGetRepairOrderLimits(ctx context.Context, repairer *identity.PeerIdentity, pointer *pb.Pointer, healthy []*pb.RemotePiece) ([]*pb.AddressedOrderLimit, error) {
	rootPieceID := pointer.GetRemote().RootPieceId
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
	totalPieces := pointer.GetRemote().GetRedundancy().GetTotal()
	expiration := pointer.ExpirationDate

	bucketPath := storj.Path("TODO") // TODO:
	serialNumber, err := service.createSerial(ctx, bucketPath)
	if err != nil {
		return nil, err
	}
	// defer service.saveSerial(ctx, serialNumber, ...)

	// convert orderExpiration from duration to timestamp
	orderExpiration, err := ptypes.TimestampProto(time.Now().Add(service.orderExpiration))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range healthy {
		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			// TODO: audit should not fail if a single node cannot be retrieved from overlay cache or is offline
			// TODO: undo serial entry
			return nil, Error.Wrap(err)
		}

		if node != nil {
			node.Type.DPanicOnInvalid("repairer order limits")
		}

		orderLimit, err := signing.SignOrderLimit(service.satellite, &pb.OrderLimit2{
			SerialNumber:    serialNumber,
			SatelliteId:     service.satellite.ID(),
			UplinkId:        repairer.ID,
			StorageNodeId:   piece.NodeId,
			PieceId:         rootPieceID.Derive(piece.NodeId),
			Action:          pb.PieceAction_GET_REPAIR,
			Limit:           int64(shareSize),
			PieceExpiration: expiration,
			OrderExpiration: orderExpiration,
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
	}

	return limits, nil
}

func (service *Service) CreatePutRepairOrderLimits(ctx context.Context, repairer *identity.PeerIdentity, pointer *pb.Pointer, getOrderLimits []*pb.AddressedOrderLimit, newNodes []*pb.Node) ([]*pb.AddressedOrderLimit, error) {
	rootPieceID := pointer.GetRemote().RootPieceId
	shareSize := pointer.GetRemote().GetRedundancy().GetErasureShareSize()
	totalPieces := pointer.GetRemote().GetRedundancy().GetTotal()
	expiration := pointer.ExpirationDate

	bucketPath := storj.Path("TODO") // TODO:
	serialNumber, err := service.createSerial(ctx, bucketPath)
	if err != nil {
		return nil, err
	}
	// defer service.saveSerial(ctx, serialNumber, ...)

	// convert orderExpiration from days to timstamp
	orderExpiration, err := ptypes.TimestampProto(time.Now().Add(service.orderExpiration))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	var pieceNum int32
	for _, node := range newNodes {
		if node != nil {
			node.Type.DPanicOnInvalid("repair 2")
		}

		for pieceNum < totalPieces && getOrderLimits[pieceNum] != nil {
			pieceNum++
		}

		if pieceNum >= totalPieces { // should not happen
			return nil, Error.New("piece num greater than total pieces: %d >= %d", pieceNum, totalPieces)
		}

		orderLimit, err := signing.SignOrderLimit(service.satellite, &pb.OrderLimit2{
			SerialNumber:    serialNumber,
			SatelliteId:     service.satellite.ID(),
			UplinkId:        repairer.ID,
			StorageNodeId:   node.Id,
			PieceId:         rootPieceID.Derive(node.Id),
			Action:          pb.PieceAction_PUT_REPAIR,
			Limit:           int64(shareSize),
			PieceExpiration: expiration,
			OrderExpiration: orderExpiration,
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits[pieceNum] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		pieceNum++
	}

	return limits, nil
}
