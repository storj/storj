// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"bytes"
	"context"
	"math"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink/eestream"
)

// ErrDownloadFailedNotEnoughPieces is returned when download failed due to missing pieces
var ErrDownloadFailedNotEnoughPieces = errs.Class("not enough pieces for download")

// Config is a configuration struct for orders Service.
type Config struct {
	Expiration          time.Duration `help:"how long until an order expires" default:"168h"` // 7 days
	SettlementBatchSize int           `help:"how many orders to batch per transaction" default:"250"`
}

// Service for creating order limits.
//
// architecture: Service
type Service struct {
	log                                 *zap.Logger
	satellite                           signing.Signer
	overlay                             *overlay.Service
	orders                              DB
	satelliteAddress                    *pb.NodeAddress
	orderExpiration                     time.Duration
	repairMaxExcessRateOptimalThreshold float64
}

// NewService creates new service for creating order limits.
func NewService(
	log *zap.Logger, satellite signing.Signer, overlay *overlay.Service,
	orders DB, orderExpiration time.Duration, satelliteAddress *pb.NodeAddress,
	repairMaxExcessRateOptimalThreshold float64,
) *Service {
	return &Service{
		log:                                 log,
		satellite:                           satellite,
		overlay:                             overlay,
		orders:                              orders,
		satelliteAddress:                    satelliteAddress,
		orderExpiration:                     orderExpiration,
		repairMaxExcessRateOptimalThreshold: repairMaxExcessRateOptimalThreshold,
	}
}

// VerifyOrderLimitSignature verifies that the signature inside order limit belongs to the satellite.
func (service *Service) VerifyOrderLimitSignature(ctx context.Context, signed *pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)
	return signing.VerifyOrderLimitSignature(ctx, service.satellite, signed)
}

func (service *Service) createSerial(ctx context.Context) (_ storj.SerialNumber, err error) {
	defer mon.Task()(&ctx)(&err)
	uuid, err := uuid.New()
	if err != nil {
		return storj.SerialNumber{}, Error.Wrap(err)
	}
	return storj.SerialNumber(*uuid), nil
}

func (service *Service) saveSerial(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, expiresAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.orders.CreateSerialInfo(ctx, serialNumber, bucketID, expiresAt)
}

func (service *Service) updateBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, addressedOrderLimits ...*pb.AddressedOrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(addressedOrderLimits) == 0 {
		return nil
	}

	var action pb.PieceAction

	var bucketAllocation int64
	var nodesAllocation int64
	nodes := make([]storj.NodeID, 0, len(addressedOrderLimits))

	for _, addressedOrderLimit := range addressedOrderLimits {
		if addressedOrderLimit != nil {
			orderLimit := addressedOrderLimit.Limit

			if nodesAllocation == 0 {
				nodesAllocation = orderLimit.Limit
			} else if nodesAllocation != orderLimit.Limit {
				return Error.New("inconsistent allocations had %d got %d", nodesAllocation, orderLimit.Limit)
			}

			nodes = append(nodes, orderLimit.StorageNodeId)
			action = orderLimit.Action

			bucketAllocation += orderLimit.Limit
		}
	}

	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// TODO: all of this below should be a single db transaction. in fact, this whole function should probably be part of an existing transaction
	if err := service.orders.UpdateBucketBandwidthAllocation(ctx, projectID, bucketName, action, bucketAllocation, intervalStart); err != nil {
		return Error.Wrap(err)
	}

	if err := service.orders.UpdateStoragenodeBandwidthAllocation(ctx, nodes, action, nodesAllocation, intervalStart); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// CreateGetOrderLimits creates the order limits for downloading the pieces of pointer.
func (service *Service) CreateGetOrderLimits(ctx context.Context, bucketID []byte, pointer *pb.Pointer) (_ []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	rootPieceID := pointer.GetRemote().RootPieceId
	pieceExpiration := pointer.ExpirationDate
	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)

	var combinedErrs error
	var limits []*pb.AddressedOrderLimit
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		node, err := service.overlay.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Debug("error getting node from overlay", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New("%v", node.Id))
			continue
		}

		if !service.overlay.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New("%v", node.Id))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkPublicKey:  piecePublicKey,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId, piece.PieceNum),
			Action:           pb.PieceAction_GET,
			Limit:            pieceSize,
			PieceExpiration:  pieceExpiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		limits = append(limits, &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		})
	}

	if len(limits) < redundancy.RequiredCount() {
		mon.Meter("download_failed_not_enough_pieces_uplink").Mark(1) //locked
		err = Error.New("not enough nodes available: got %d, required %d", len(limits), redundancy.RequiredCount())
		return nil, storj.PiecePrivateKey{}, ErrDownloadFailedNotEnoughPieces.Wrap(errs.Combine(err, combinedErrs))
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limits, piecePrivateKey, nil
}

// CreatePutOrderLimits creates the order limits for uploading pieces to nodes.
func (service *Service) CreatePutOrderLimits(ctx context.Context, bucketID []byte, nodes []*pb.Node, expiration time.Time, maxPieceSize int64) (_ storj.PieceID, _ []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	rootPieceID := storj.NewPieceID()
	limits := make([]*pb.AddressedOrderLimit, len(nodes))
	var pieceNum int32
	for _, node := range nodes {
		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkPublicKey:  piecePublicKey,
			StorageNodeId:    node.Id,
			PieceId:          rootPieceID.Derive(node.Id, pieceNum),
			Action:           pb.PieceAction_PUT,
			Limit:            maxPieceSize,
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		limits[pieceNum] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		pieceNum++
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return storj.PieceID{}, nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return rootPieceID, limits, piecePrivateKey, nil
}

// CreateDeleteOrderLimits creates the order limits for deleting the pieces of pointer.
func (service *Service) CreateDeleteOrderLimits(ctx context.Context, bucketID []byte, pointer *pb.Pointer) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	rootPieceID := pointer.GetRemote().RootPieceId
	pieceExpiration := pointer.ExpirationDate
	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	var combinedErrs error
	var limits []*pb.AddressedOrderLimit
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		node, err := service.overlay.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Error("error getting node from overlay", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New("%v", node.Id))
			continue
		}

		if !service.overlay.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New("%v", node.Id))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkPublicKey:  piecePublicKey,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId, piece.PieceNum),
			Action:           pb.PieceAction_DELETE,
			Limit:            0,
			PieceExpiration:  pieceExpiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		limits = append(limits, &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		})
	}

	if len(limits) == 0 {
		err = Error.New("failed creating order limits for all nodes")
		return nil, storj.PiecePrivateKey{}, errs.Combine(err, combinedErrs)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limits, piecePrivateKey, nil
}

// CreateAuditOrderLimits creates the order limits for auditing the pieces of pointer.
func (service *Service) CreateAuditOrderLimits(ctx context.Context, bucketID []byte, pointer *pb.Pointer, skip map[storj.NodeID]bool) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)
	rootPieceID := pointer.GetRemote().RootPieceId
	redundancy := pointer.GetRemote().GetRedundancy()
	shareSize := redundancy.GetErasureShareSize()
	totalPieces := redundancy.GetTotal()

	pieceExpiration := pointer.ExpirationDate
	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	var combinedErrs error
	var limitsCount int32
	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if skip[piece.NodeId] {
			continue
		}

		node, err := service.overlay.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Error("error getting node from overlay", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New("%v", node.Id))
			continue
		}

		if !service.overlay.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New("%v", node.Id))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkPublicKey:  piecePublicKey,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId, piece.PieceNum),
			Action:           pb.PieceAction_GET_AUDIT,
			Limit:            int64(shareSize),
			PieceExpiration:  pieceExpiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		limitsCount++
	}

	if limitsCount < redundancy.GetMinReq() {
		err = Error.New("not enough nodes available: got %d, required %d", limitsCount, redundancy.GetMinReq())
		return nil, storj.PiecePrivateKey{}, errs.Combine(err, combinedErrs)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limits, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limits, piecePrivateKey, nil
}

// CreateAuditOrderLimit creates an order limit for auditing a single the piece from a pointer.
func (service *Service) CreateAuditOrderLimit(ctx context.Context, bucketID []byte, nodeID storj.NodeID, pieceNum int32, rootPieceID storj.PieceID, shareSize int32) (limit *pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	// TODO reduce number of params ?
	defer mon.Task()(&ctx)(&err)

	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	node, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	if node.Disqualified != nil {
		return nil, storj.PiecePrivateKey{}, overlay.ErrNodeDisqualified.New("%v", nodeID)
	}

	if !service.overlay.IsOnline(node) {
		return nil, storj.PiecePrivateKey{}, overlay.ErrNodeOffline.New("%v", nodeID)
	}

	orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
		SerialNumber:     serialNumber,
		SatelliteId:      service.satellite.ID(),
		SatelliteAddress: service.satelliteAddress,
		UplinkPublicKey:  piecePublicKey,
		StorageNodeId:    nodeID,
		PieceId:          rootPieceID.Derive(nodeID, pieceNum),
		Action:           pb.PieceAction_GET_AUDIT,
		Limit:            int64(shareSize),
		OrderCreation:    time.Now(),
		OrderExpiration:  orderExpiration,
	})
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	limit = &pb.AddressedOrderLimit{
		Limit:              orderLimit,
		StorageNodeAddress: node.Address,
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limit, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limit); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limit, piecePrivateKey, nil
}

// CreateGetRepairOrderLimits creates the order limits for downloading the
// healthy pieces of pointer as the source for repair.
//
// The length of the returned orders slice is the total number of pieces of the
// segment, setting to null the ones which don't correspond to a healthy piece.
// CreateGetRepairOrderLimits creates the order limits for downloading the healthy pieces of pointer as the source for repair.
func (service *Service) CreateGetRepairOrderLimits(ctx context.Context, bucketID []byte, pointer *pb.Pointer, healthy []*pb.RemotePiece) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	rootPieceID := pointer.GetRemote().RootPieceId
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	totalPieces := redundancy.TotalCount()
	pieceExpiration := pointer.ExpirationDate
	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	var combinedErrs error
	var limitsCount int
	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range healthy {
		node, err := service.overlay.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Error("error getting node from the overlay", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New("%v", node.Id))
			continue
		}

		if !service.overlay.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New("%v", node.Id))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkPublicKey:  piecePublicKey,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId, piece.PieceNum),
			Action:           pb.PieceAction_GET_REPAIR,
			Limit:            pieceSize,
			PieceExpiration:  pieceExpiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		limitsCount++
	}

	if limitsCount < redundancy.RequiredCount() {
		err = Error.New("not enough nodes available: got %d, required %d", limitsCount, redundancy.RequiredCount())
		return nil, storj.PiecePrivateKey{}, errs.Combine(err, combinedErrs)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limits, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limits, piecePrivateKey, nil
}

// CreatePutRepairOrderLimits creates the order limits for uploading the repaired pieces of pointer to newNodes.
func (service *Service) CreatePutRepairOrderLimits(ctx context.Context, bucketID []byte, pointer *pb.Pointer, getOrderLimits []*pb.AddressedOrderLimit, newNodes []*pb.Node) (_ []*pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)
	orderExpiration := time.Now().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	var limits []*pb.AddressedOrderLimit
	{ // Create the order limits for being used to upload the repaired pieces
		redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
		if err != nil {
			return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
		}

		totalPieces := redundancy.TotalCount()
		limits = make([]*pb.AddressedOrderLimit, totalPieces)

		totalPiecesAfterRepair := int(
			math.Ceil(
				float64(redundancy.OptimalThreshold()) * (1 + service.repairMaxExcessRateOptimalThreshold),
			),
		)
		if totalPiecesAfterRepair > totalPieces {
			totalPiecesAfterRepair = totalPieces
		}

		var numCurrentPieces int
		for _, o := range getOrderLimits {
			if o != nil {
				numCurrentPieces++
			}
		}

		var (
			totalPiecesToRepair = totalPiecesAfterRepair - numCurrentPieces
			rootPieceID         = pointer.GetRemote().RootPieceId
			pieceSize           = eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
			pieceNum            int32
		)
		for _, node := range newNodes {
			for int(pieceNum) < totalPieces && getOrderLimits[pieceNum] != nil {
				pieceNum++
			}

			if int(pieceNum) >= totalPieces { // should not happen
				return nil, storj.PiecePrivateKey{}, Error.New("piece num greater than total pieces: %d >= %d", pieceNum, totalPieces)
			}

			orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
				SerialNumber:     serialNumber,
				SatelliteId:      service.satellite.ID(),
				SatelliteAddress: service.satelliteAddress,
				UplinkPublicKey:  piecePublicKey,
				StorageNodeId:    node.Id,
				PieceId:          rootPieceID.Derive(node.Id, pieceNum),
				Action:           pb.PieceAction_PUT_REPAIR,
				Limit:            pieceSize,
				PieceExpiration:  pointer.ExpirationDate,
				OrderCreation:    time.Now(),
				OrderExpiration:  orderExpiration,
			})
			if err != nil {
				return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
			}

			limits[pieceNum] = &pb.AddressedOrderLimit{
				Limit:              orderLimit,
				StorageNodeAddress: node.Address,
			}
			pieceNum++
			totalPiecesToRepair--

			if totalPiecesToRepair == 0 {
				break
			}
		}
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limits, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limits, piecePrivateKey, nil
}

// CreateGracefulExitPutOrderLimit creates an order limit for graceful exit put transfers.
func (service *Service) CreateGracefulExitPutOrderLimit(ctx context.Context, bucketID []byte, nodeID storj.NodeID, pieceNum int32, rootPieceID storj.PieceID, shareSize int32) (limit *pb.AddressedOrderLimit, _ storj.PiecePrivateKey, err error) {
	defer mon.Task()(&ctx)(&err)

	orderExpiration := time.Now().UTC().Add(service.orderExpiration)

	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	node, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	if node.Disqualified != nil {
		return nil, storj.PiecePrivateKey{}, overlay.ErrNodeDisqualified.New("%v", nodeID)
	}

	if !service.overlay.IsOnline(node) {
		return nil, storj.PiecePrivateKey{}, overlay.ErrNodeOffline.New("%v", nodeID)
	}

	orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
		SerialNumber:     serialNumber,
		SatelliteId:      service.satellite.ID(),
		SatelliteAddress: service.satelliteAddress,
		UplinkPublicKey:  piecePublicKey,
		StorageNodeId:    nodeID,
		PieceId:          rootPieceID.Derive(nodeID, pieceNum),
		Action:           pb.PieceAction_PUT,
		Limit:            int64(shareSize),
		OrderCreation:    time.Now().UTC(),
		OrderExpiration:  orderExpiration,
	})
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	limit = &pb.AddressedOrderLimit{
		Limit:              orderLimit,
		StorageNodeAddress: node.Address,
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpiration)
	if err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limit, storj.PiecePrivateKey{}, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limit); err != nil {
		return nil, storj.PiecePrivateKey{}, Error.Wrap(err)
	}

	return limit, piecePrivateKey, nil
}

// UpdateGetInlineOrder updates amount of inline GET bandwidth for given bucket
func (service *Service) UpdateGetInlineOrder(ctx context.Context, projectID uuid.UUID, bucketName []byte, amount int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	return service.orders.UpdateBucketBandwidthInline(ctx, projectID, bucketName, pb.PieceAction_GET, amount, intervalStart)
}

// UpdatePutInlineOrder updates amount of inline PUT bandwidth for given bucket
func (service *Service) UpdatePutInlineOrder(ctx context.Context, projectID uuid.UUID, bucketName []byte, amount int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	return service.orders.UpdateBucketBandwidthInline(ctx, projectID, bucketName, pb.PieceAction_PUT, amount, intervalStart)
}

// SplitBucketID takes a bucketID, splits on /, and returns a projectID and bucketName
func SplitBucketID(bucketID []byte) (projectID *uuid.UUID, bucketName []byte, err error) {
	pathElements := bytes.Split(bucketID, []byte("/"))
	if len(pathElements) > 1 {
		bucketName = pathElements[1]
	}
	projectID, err = uuid.Parse(string(pathElements[0]))
	if err != nil {
		return nil, nil, err
	}
	return projectID, bucketName, nil
}
