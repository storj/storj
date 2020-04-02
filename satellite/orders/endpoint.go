// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"bytes"
	"context"
	"io"
	"sort"
	"time"

	monkit "github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/pb/pbgrpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// DB implements saving order after receiving from storage node
//
// architecture: Database
type DB interface {
	// CreateSerialInfo creates serial number entry in database.
	CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) error
	// UseSerialNumber creates a used serial number entry in database from an
	// existing serial number.
	// It returns the bucket ID associated to serialNumber.
	UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) ([]byte, error)
	// UnuseSerialNumber removes pair serial number -> storage node id from database
	UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) error
	// DeleteExpiredSerials deletes all expired serials in serial_number, used_serials, and consumed_serials table.
	DeleteExpiredSerials(ctx context.Context, now time.Time) (_ int, err error)
	// DeleteExpiredConsumedSerials deletes all expired serials in the consumed_serials table.
	DeleteExpiredConsumedSerials(ctx context.Context, now time.Time) (_ int, err error)

	// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
	UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
	UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
	UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// GetBucketBandwidth gets total bucket bandwidth from period of time
	GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (int64, error)
	// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
	GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error)

	// ProcessOrders takes a list of order requests and processes them in a batch
	ProcessOrders(ctx context.Context, requests []*ProcessOrderRequest) (responses []*ProcessOrderResponse, err error)

	// WithTransaction runs the callback and provides it with a Transaction.
	WithTransaction(ctx context.Context, cb func(ctx context.Context, tx Transaction) error) error
	// WithQueue runs the callback and provides it with a Queue. When the callback returns with
	// no error, any pending serials returned by the queue are removed from it.
	WithQueue(ctx context.Context, cb func(ctx context.Context, queue Queue) error) error
}

// Transaction represents a database transaction but with higher level actions.
type Transaction interface {
	// UpdateBucketBandwidthBatch updates all the bandwidth rollups in the database
	UpdateBucketBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []BucketBandwidthRollup) error

	// UpdateStoragenodeBandwidthBatch updates all the bandwidth rollups in the database
	UpdateStoragenodeBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []StoragenodeBandwidthRollup) error

	// CreateConsumedSerialsBatch creates the batch of ConsumedSerials.
	CreateConsumedSerialsBatch(ctx context.Context, consumedSerials []ConsumedSerial) (err error)

	// HasConsumedSerial returns true if the node and serial number have been consumed.
	HasConsumedSerial(ctx context.Context, nodeID storj.NodeID, serialNumber storj.SerialNumber) (bool, error)
}

// Queue is an abstraction around a queue of pending serials.
type Queue interface {
	// GetPendingSerialsBatch returns a batch of pending serials containing at most size
	// entries. It returns a boolean indicating true if the queue is empty.
	GetPendingSerialsBatch(ctx context.Context, size int) ([]PendingSerial, bool, error)
}

// ConsumedSerial is a serial that has been consumed and its bandwidth recorded.
type ConsumedSerial struct {
	NodeID       storj.NodeID
	SerialNumber storj.SerialNumber
	ExpiresAt    time.Time
}

// PendingSerial is a serial number reported by a storagenode waiting to be
// settled
type PendingSerial struct {
	NodeID       storj.NodeID
	BucketID     []byte
	Action       uint
	SerialNumber storj.SerialNumber
	ExpiresAt    time.Time
	Settled      uint64
}

var (
	// Error the default orders errs class
	Error = errs.Class("orders error")
	// ErrUsingSerialNumber error class for serial number
	ErrUsingSerialNumber = errs.Class("serial number")

	errExpiredOrder = errs.Class("order limit expired")

	mon = monkit.Package()
)

// BucketBandwidthRollup contains all the info needed for a bucket bandwidth rollup
type BucketBandwidthRollup struct {
	ProjectID  uuid.UUID
	BucketName string
	Action     pb.PieceAction
	Inline     int64
	Allocated  int64
	Settled    int64
}

// SortBucketBandwidthRollups sorts the rollups
func SortBucketBandwidthRollups(rollups []BucketBandwidthRollup) {
	sort.SliceStable(rollups, func(i, j int) bool {
		uuidCompare := bytes.Compare(rollups[i].ProjectID[:], rollups[j].ProjectID[:])
		switch {
		case uuidCompare == -1:
			return true
		case uuidCompare == 1:
			return false
		case rollups[i].BucketName < rollups[j].BucketName:
			return true
		case rollups[i].BucketName > rollups[j].BucketName:
			return false
		case rollups[i].Action < rollups[j].Action:
			return true
		case rollups[i].Action > rollups[j].Action:
			return false
		default:
			return false
		}
	})
}

// StoragenodeBandwidthRollup contains all the info needed for a storagenode bandwidth rollup
type StoragenodeBandwidthRollup struct {
	NodeID    storj.NodeID
	Action    pb.PieceAction
	Allocated int64
	Settled   int64
}

// SortStoragenodeBandwidthRollups sorts the rollups
func SortStoragenodeBandwidthRollups(rollups []StoragenodeBandwidthRollup) {
	sort.SliceStable(rollups, func(i, j int) bool {
		nodeCompare := bytes.Compare(rollups[i].NodeID.Bytes(), rollups[j].NodeID.Bytes())
		switch {
		case nodeCompare == -1:
			return true
		case nodeCompare == 1:
			return false
		case rollups[i].Action < rollups[j].Action:
			return true
		case rollups[i].Action > rollups[j].Action:
			return false
		default:
			return false
		}
	})
}

// ProcessOrderRequest for batch order processing
type ProcessOrderRequest struct {
	Order      *pb.Order
	OrderLimit *pb.OrderLimit
}

// ProcessOrderResponse for batch order processing responses
type ProcessOrderResponse struct {
	SerialNumber storj.SerialNumber
	Status       pb.SettlementResponse_Status
}

// Endpoint for orders receiving
//
// architecture: Endpoint
type Endpoint struct {
	log                 *zap.Logger
	satelliteSignee     signing.Signee
	DB                  DB
	settlementBatchSize int
}

// drpcEndpoint wraps streaming methods so that they can be used with drpc
type drpcEndpoint struct{ *Endpoint }

// DRPC returns a DRPC form of the endpoint.
func (endpoint *Endpoint) DRPC() pb.DRPCOrdersServer { return &drpcEndpoint{Endpoint: endpoint} }

// NewEndpoint new orders receiving endpoint
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, db DB, settlementBatchSize int) *Endpoint {
	return &Endpoint{
		log:                 log,
		satelliteSignee:     satelliteSignee,
		DB:                  db,
		settlementBatchSize: settlementBatchSize,
	}
}

func monitoredSettlementStreamReceive(ctx context.Context, stream settlementStream) (_ *pb.SettlementRequest, err error) {
	defer mon.Task()(&ctx)(&err)
	return stream.Recv()
}

func monitoredSettlementStreamSend(ctx context.Context, stream settlementStream, resp *pb.SettlementResponse) (err error) {
	defer mon.Task()(&ctx)(&err)
	switch resp.Status {
	case pb.SettlementResponse_ACCEPTED:
		mon.Event("settlement_response_accepted")
	case pb.SettlementResponse_REJECTED:
		mon.Event("settlement_response_rejected")
	default:
		mon.Event("settlement_response_unknown")
	}
	return stream.Send(resp)
}

// Settlement receives orders and handles them in batches
func (endpoint *Endpoint) Settlement(stream pbgrpc.Orders_SettlementServer) (err error) {
	return endpoint.doSettlement(stream)
}

// Settlement receives orders and handles them in batches
func (endpoint *drpcEndpoint) Settlement(stream pb.DRPCOrders_SettlementStream) (err error) {
	return endpoint.doSettlement(stream)
}

// settlementStream is the minimum interface required to perform settlements.
type settlementStream interface {
	Context() context.Context
	Send(*pb.SettlementResponse) error
	Recv() (*pb.SettlementRequest, error)
}

// doSettlement receives orders and handles them in batches
func (endpoint *Endpoint) doSettlement(stream settlementStream) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	formatError := func(err error) error {
		if err == io.EOF {
			return nil
		}
		return rpcstatus.Error(rpcstatus.Unknown, err.Error())
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("Settlement")

	requests := make([]*ProcessOrderRequest, 0, endpoint.settlementBatchSize)

	defer func() {
		if len(requests) > 0 {
			err = errs.Combine(err, endpoint.processOrders(ctx, stream, requests))
			if err != nil {
				err = formatError(err)
			}
		}
	}()

	var expirationCount int64
	defer func() {
		if expirationCount > 0 {
			log.Debug("order verification found expired orders", zap.Int64("amount", expirationCount))
		}
	}()

	for {
		request, err := monitoredSettlementStreamReceive(ctx, stream)
		if err != nil {
			return formatError(err)
		}

		if request == nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, "request missing")
		}
		if request.Limit == nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, "order limit missing")
		}
		if request.Order == nil {
			return rpcstatus.Error(rpcstatus.InvalidArgument, "order missing")
		}

		orderLimit := request.Limit
		order := request.Order

		if orderLimit.StorageNodeId != peer.ID {
			return rpcstatus.Error(rpcstatus.Unauthenticated, "only specified storage node can settle order")
		}

		rejectErr := func() error {
			// satellite verifies that it signed the order limit
			if err := signing.VerifyOrderLimitSignature(ctx, endpoint.satelliteSignee, orderLimit); err != nil {
				mon.Event("order_verification_failed_satellite_signature")
				return Error.New("unable to verify order limit")
			}

			// satellite verifies that the order signature matches pub key in order limit
			if err := signing.VerifyUplinkOrderSignature(ctx, orderLimit.UplinkPublicKey, order); err != nil {
				mon.Event("order_verification_failed_uplink_signature")
				return Error.New("unable to verify order")
			}

			// TODO should this reject or just error ??
			if orderLimit.SerialNumber != order.SerialNumber {
				mon.Event("order_verification_failed_serial_mismatch")
				return Error.New("invalid serial number")
			}

			if orderLimit.OrderExpiration.Before(time.Now()) {
				mon.Event("order_verification_failed_expired")
				expirationCount++
				return errExpiredOrder.New("order limit expired")
			}
			return nil
		}()
		if rejectErr != nil {
			mon.Event("order_verification_failed")
			if !errExpiredOrder.Has(rejectErr) {
				log.Debug("order limit/order verification failed", zap.Stringer("serial", orderLimit.SerialNumber), zap.Error(rejectErr))
			}
			err := monitoredSettlementStreamSend(ctx, stream, &pb.SettlementResponse{
				SerialNumber: orderLimit.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			if err != nil {
				return formatError(err)
			}
			continue
		}

		requests = append(requests, &ProcessOrderRequest{Order: order, OrderLimit: orderLimit})

		if len(requests) >= endpoint.settlementBatchSize {
			err = endpoint.processOrders(ctx, stream, requests)
			requests = requests[:0]
			if err != nil {
				return formatError(err)
			}
		}
	}
}

func (endpoint *Endpoint) processOrders(ctx context.Context, stream settlementStream, requests []*ProcessOrderRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	responses, err := endpoint.DB.ProcessOrders(ctx, requests)
	if err != nil {
		return err
	}

	for _, response := range responses {
		r := &pb.SettlementResponse{
			SerialNumber: response.SerialNumber,
			Status:       response.Status,
		}
		err = monitoredSettlementStreamSend(ctx, stream, r)
		if err != nil {
			return err
		}
	}
	return nil
}
