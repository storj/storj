// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"bytes"
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Config is a configuration struct for orders Service.
type Config struct {
	Expiration time.Duration `help:"how long until an order expires" default:"1080h"`
}

// Service for creating order limits.
type Service struct {
	log              *zap.Logger
	satellite        signing.Signer
	cache            *overlay.Cache
	certdb           certdb.DB
	orders           DB
	satelliteAddress *pb.NodeAddress

	orderExpiration time.Duration
}

// NewService creates new service for creating order limits.
func NewService(log *zap.Logger, satellite signing.Signer, cache *overlay.Cache, certdb certdb.DB, orders DB, orderExpiration time.Duration, satelliteAddress *pb.NodeAddress) *Service {
	return &Service{
		log:              log,
		satellite:        satellite,
		cache:            cache,
		certdb:           certdb,
		orders:           orders,
		satelliteAddress: satelliteAddress,
		orderExpiration:  orderExpiration,
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
func (service *Service) CreateGetOrderLimits(ctx context.Context, uplink *identity.PeerIdentity, bucketID []byte, pointer *pb.Pointer) (_ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	rootPieceID := pointer.GetRemote().RootPieceId
	expiration := pointer.ExpirationDate

	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, err
	}

	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)

	var combinedErrs error
	var limits []*pb.AddressedOrderLimit
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Debug("error getting node from overlay cache", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New(node.Id.String()))
			continue
		}

		if !service.cache.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New(node.Id.String()))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkId:         uplink.ID,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId),
			Action:           pb.PieceAction_GET,
			Limit:            pieceSize,
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits = append(limits, &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		})
	}

	if len(limits) < redundancy.RequiredCount() {
		err = Error.New("not enough nodes available: got %d, required %d", len(limits), redundancy.RequiredCount())
		return nil, errs.Combine(err, combinedErrs)
	}

	err = service.certdb.SavePublicKey(ctx, uplink.ID, uplink.Leaf.PublicKey)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, Error.Wrap(err)
	}

	return limits, nil
}

// CreatePutOrderLimits creates the order limits for uploading pieces to nodes.
func (service *Service) CreatePutOrderLimits(ctx context.Context, uplink *identity.PeerIdentity, bucketID []byte, nodes []*pb.Node, expiration *timestamp.Timestamp, maxPieceSize int64) (_ storj.PieceID, _ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return storj.PieceID{}, nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return storj.PieceID{}, nil, err
	}

	rootPieceID := storj.NewPieceID()
	limits := make([]*pb.AddressedOrderLimit, len(nodes))
	var pieceNum int32
	for _, node := range nodes {
		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkId:         uplink.ID,
			StorageNodeId:    node.Id,
			PieceId:          rootPieceID.Derive(node.Id),
			Action:           pb.PieceAction_PUT,
			Limit:            maxPieceSize,
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
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

	err = service.certdb.SavePublicKey(ctx, uplink.ID, uplink.Leaf.PublicKey)
	if err != nil {
		return storj.PieceID{}, nil, Error.Wrap(err)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return storj.PieceID{}, nil, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return rootPieceID, limits, err
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return storj.PieceID{}, nil, Error.Wrap(err)
	}

	return rootPieceID, limits, nil
}

// CreateDeleteOrderLimits creates the order limits for deleting the pieces of pointer.
func (service *Service) CreateDeleteOrderLimits(ctx context.Context, uplink *identity.PeerIdentity, bucketID []byte, pointer *pb.Pointer) (_ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	rootPieceID := pointer.GetRemote().RootPieceId
	expiration := pointer.ExpirationDate

	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, err
	}

	var combinedErrs error
	var limits []*pb.AddressedOrderLimit
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Error("error getting node from overlay cache", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New(node.Id.String()))
			continue
		}

		if !service.cache.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New(node.Id.String()))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkId:         uplink.ID,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId),
			Action:           pb.PieceAction_DELETE,
			Limit:            0,
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits = append(limits, &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		})
	}

	if len(limits) == 0 {
		err = Error.New("failed creating order limits for all nodes")
		return nil, errs.Combine(err, combinedErrs)
	}

	err = service.certdb.SavePublicKey(ctx, uplink.ID, uplink.Leaf.PublicKey)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return limits, nil
}

// CreateAuditOrderLimits creates the order limits for auditing the pieces of pointer.
func (service *Service) CreateAuditOrderLimits(ctx context.Context, auditor *identity.PeerIdentity, bucketID []byte, pointer *pb.Pointer, skip map[storj.NodeID]bool) (_ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	rootPieceID := pointer.GetRemote().RootPieceId
	redundancy := pointer.GetRemote().GetRedundancy()
	shareSize := redundancy.GetErasureShareSize()
	totalPieces := redundancy.GetTotal()
	expiration := pointer.ExpirationDate

	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, err
	}

	var combinedErrs error
	var limitsCount int32
	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		if skip[piece.NodeId] {
			continue
		}

		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Error("error getting node from the overlay cache", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New(node.Id.String()))
			continue
		}

		if !service.cache.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New(node.Id.String()))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkId:         auditor.ID,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId),
			Action:           pb.PieceAction_GET_AUDIT,
			Limit:            int64(shareSize),
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		limitsCount++
	}

	if limitsCount < redundancy.GetMinReq() {
		err = Error.New("not enough nodes available: got %d, required %d", limitsCount, redundancy.GetMinReq())
		return nil, errs.Combine(err, combinedErrs)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limits, err
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, Error.Wrap(err)
	}

	return limits, nil
}

// CreateAuditOrderLimit creates an order limit for auditing a single the piece from a pointer.
func (service *Service) CreateAuditOrderLimit(ctx context.Context, auditor *identity.PeerIdentity, bucketID []byte, nodeID storj.NodeID, rootPieceID storj.PieceID, shareSize int32) (limit *pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, err
	}

	node, err := service.cache.Get(ctx, nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if node.Disqualified != nil {
		return nil, overlay.ErrNodeDisqualified.New(nodeID.String())
	}

	if !service.cache.IsOnline(node) {
		return nil, overlay.ErrNodeOffline.New(nodeID.String())
	}

	orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
		SerialNumber:     serialNumber,
		SatelliteId:      service.satellite.ID(),
		SatelliteAddress: service.satelliteAddress,
		UplinkId:         auditor.ID,
		StorageNodeId:    nodeID,
		PieceId:          rootPieceID.Derive(nodeID),
		Action:           pb.PieceAction_GET_AUDIT,
		Limit:            int64(shareSize),
		OrderCreation:    time.Now(),
		OrderExpiration:  orderExpiration,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	limit = &pb.AddressedOrderLimit{
		Limit:              orderLimit,
		StorageNodeAddress: node.Address,
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limit, err
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limit); err != nil {
		return nil, Error.Wrap(err)
	}

	return limit, nil
}

// CreateGetRepairOrderLimits creates the order limits for downloading the healthy pieces of pointer as the source for repair.
func (service *Service) CreateGetRepairOrderLimits(ctx context.Context, repairer *identity.PeerIdentity, bucketID []byte, pointer *pb.Pointer, healthy []*pb.RemotePiece) (_ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	rootPieceID := pointer.GetRemote().RootPieceId
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	totalPieces := redundancy.TotalCount()
	expiration := pointer.ExpirationDate

	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, err
	}

	var combinedErrs error
	var limitsCount int
	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	for _, piece := range healthy {
		node, err := service.cache.Get(ctx, piece.NodeId)
		if err != nil {
			service.log.Error("error getting node from the overlay cache", zap.Error(err))
			combinedErrs = errs.Combine(combinedErrs, err)
			continue
		}

		if node.Disqualified != nil {
			service.log.Debug("node is disqualified", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeDisqualified.New(node.Id.String()))
			continue
		}

		if !service.cache.IsOnline(node) {
			service.log.Debug("node is offline", zap.Stringer("ID", node.Id))
			combinedErrs = errs.Combine(combinedErrs, overlay.ErrNodeOffline.New(node.Id.String()))
			continue
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkId:         repairer.ID,
			StorageNodeId:    piece.NodeId,
			PieceId:          rootPieceID.Derive(piece.NodeId),
			Action:           pb.PieceAction_GET_REPAIR,
			Limit:            pieceSize,
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}

		limits[piece.GetPieceNum()] = &pb.AddressedOrderLimit{
			Limit:              orderLimit,
			StorageNodeAddress: node.Address,
		}
		limitsCount++
	}

	if limitsCount < redundancy.RequiredCount() {
		err = Error.New("not enough nodes available: got %d, required %d", limitsCount, redundancy.RequiredCount())
		return nil, errs.Combine(err, combinedErrs)
	}

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limits, err
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, Error.Wrap(err)
	}

	return limits, nil
}

// CreatePutRepairOrderLimits creates the order limits for uploading the repaired pieces of pointer to newNodes.
func (service *Service) CreatePutRepairOrderLimits(ctx context.Context, repairer *identity.PeerIdentity, bucketID []byte, pointer *pb.Pointer, getOrderLimits []*pb.AddressedOrderLimit, newNodes []*pb.Node) (_ []*pb.AddressedOrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	rootPieceID := pointer.GetRemote().RootPieceId
	redundancy, err := eestream.NewRedundancyStrategyFromProto(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	pieceSize := eestream.CalcPieceSize(pointer.GetSegmentSize(), redundancy)
	totalPieces := redundancy.TotalCount()
	expiration := pointer.ExpirationDate

	// convert orderExpiration from duration to timestamp
	orderExpirationTime := time.Now().UTC().Add(service.orderExpiration)
	orderExpiration, err := ptypes.TimestampProto(orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	serialNumber, err := service.createSerial(ctx)
	if err != nil {
		return nil, err
	}

	limits := make([]*pb.AddressedOrderLimit, totalPieces)
	var pieceNum int
	for _, node := range newNodes {
		for pieceNum < totalPieces && getOrderLimits[pieceNum] != nil {
			pieceNum++
		}

		if pieceNum >= totalPieces { // should not happen
			return nil, Error.New("piece num greater than total pieces: %d >= %d", pieceNum, totalPieces)
		}

		orderLimit, err := signing.SignOrderLimit(ctx, service.satellite, &pb.OrderLimit{
			SerialNumber:     serialNumber,
			SatelliteId:      service.satellite.ID(),
			SatelliteAddress: service.satelliteAddress,
			UplinkId:         repairer.ID,
			StorageNodeId:    node.Id,
			PieceId:          rootPieceID.Derive(node.Id),
			Action:           pb.PieceAction_PUT_REPAIR,
			Limit:            pieceSize,
			PieceExpiration:  expiration,
			OrderCreation:    time.Now(),
			OrderExpiration:  orderExpiration,
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

	err = service.saveSerial(ctx, serialNumber, bucketID, orderExpirationTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectID, bucketName, err := SplitBucketID(bucketID)
	if err != nil {
		return limits, err
	}
	if err := service.updateBandwidth(ctx, *projectID, bucketName, limits...); err != nil {
		return nil, Error.Wrap(err)
	}

	return limits, nil
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
