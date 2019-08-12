// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/certdb"
)

// DB implements saving order after receiving from storage node
type DB interface {
	// CreateSerialInfo creates serial number entry in database
	CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) error
	// UseSerialNumber creates serial number entry in database
	UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) ([]byte, error)
	// UnuseSerialNumber removes pair serial number -> storage node id from database
	UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) error

	// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
	UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
	UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
	UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// UpdateStoragenodeBandwidthAllocation updates 'allocated' bandwidth for given storage nodes
	UpdateStoragenodeBandwidthAllocation(ctx context.Context, storageNodes []storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// GetBucketBandwidth gets total bucket bandwidth from period of time
	GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (int64, error)
	// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
	GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error)

	// ProcessOrders takes a list of order requests and processes them in a batch
	ProcessOrders(ctx context.Context, requests []*ProcessOrderRequest) (responses []*pb.SettlementResponse, err error)
}

var (
	// Error the default orders errs class
	Error = errs.Class("orders error")
	// ErrUsingSerialNumber error class for serial number
	ErrUsingSerialNumber = errs.Class("serial number")

	mon = monkit.Package()
)

// ProcessOrderRequest for batch order processing
type ProcessOrderRequest struct {
	Order      *pb.Order
	OrderLimit *pb.OrderLimit
}

// Endpoint for orders receiving
type Endpoint struct {
	log                 *zap.Logger
	satelliteSignee     signing.Signee
	DB                  DB
	certdb              certdb.DB
	settlementBatchSize int
}

// NewEndpoint new orders receiving endpoint
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, certdb certdb.DB, db DB, settlementBatchSize int) *Endpoint {
	return &Endpoint{
		log:                 log,
		satelliteSignee:     satelliteSignee,
		DB:                  db,
		certdb:              certdb,
		settlementBatchSize: settlementBatchSize,
	}
}

func monitoredSettlementStreamReceive(ctx context.Context, stream pb.Orders_SettlementServer) (_ *pb.SettlementRequest, err error) {
	defer mon.Task()(&ctx)(&err)
	return stream.Recv()
}

func monitoredSettlementStreamSend(ctx context.Context, stream pb.Orders_SettlementServer, resp *pb.SettlementResponse) (err error) {
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

// Settlement receives and handles orders.
func (endpoint *Endpoint) Settlement(stream pb.Orders_SettlementServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	formatError := func(err error) error {
		if err == io.EOF {
			return nil
		}
		return status.Error(codes.Unknown, err.Error())
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("Settlement")

	requests := make([]*ProcessOrderRequest, 0, endpoint.settlementBatchSize)

	defer func() {
		if len(requests) > 0 {
			err = endpoint.processOrders(ctx, stream, requests)
			if err != nil {
				err = formatError(err)
			}
		}
	}()

	for {
		request, err := monitoredSettlementStreamReceive(ctx, stream)
		if err != nil {
			return formatError(err)
		}

		if request == nil {
			return status.Error(codes.InvalidArgument, "request missing")
		}
		if request.Limit == nil {
			return status.Error(codes.InvalidArgument, "order limit missing")
		}
		if request.Order == nil {
			return status.Error(codes.InvalidArgument, "order missing")
		}

		orderLimit := request.Limit
		order := request.Order

		if orderLimit.StorageNodeId != peer.ID {
			return status.Error(codes.Unauthenticated, "only specified storage node can settle order")
		}

		rejectErr := func() error {
			if err := signing.VerifyOrderLimitSignature(ctx, endpoint.satelliteSignee, orderLimit); err != nil {
				return Error.New("unable to verify order limit")
			}

			if orderLimit.DeprecatedUplinkId == nil { // new signature handling
				if err := signing.VerifyUplinkOrderSignature(ctx, orderLimit.UplinkPublicKey, order); err != nil {
					return Error.New("unable to verify order")
				}
			} else {
				var uplinkSignee signing.Signee

				// who asked for this order: uplink (get/put/del) or satellite (get_repair/put_repair/audit)
				if endpoint.satelliteSignee.ID() == *orderLimit.DeprecatedUplinkId {
					uplinkSignee = endpoint.satelliteSignee
				} else {
					uplinkPubKey, err := endpoint.certdb.GetPublicKey(ctx, *orderLimit.DeprecatedUplinkId)
					if err != nil {
						log.Warn("unable to find uplink public key", zap.Error(err))
						return status.Errorf(codes.Internal, "unable to find uplink public key")
					}
					uplinkSignee = &signing.PublicKey{
						Self: *orderLimit.DeprecatedUplinkId,
						Key:  uplinkPubKey,
					}
				}
				if err := signing.VerifyOrderSignature(ctx, uplinkSignee, order); err != nil {
					return Error.New("unable to verify order")
				}
			}

			// TODO should this reject or just error ??
			if orderLimit.SerialNumber != order.SerialNumber {
				return Error.New("invalid serial number")
			}

			if orderLimit.OrderExpiration.Before(time.Now()) {
				return Error.New("order limit expired")
			}
			return nil
		}()
		if rejectErr != err {
			log.Debug("order limit/order verification failed", zap.Stringer("serial", orderLimit.SerialNumber), zap.Error(err), zap.Error(rejectErr))
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

		if len(requests) == endpoint.settlementBatchSize {
			err = endpoint.processOrders(ctx, stream, requests)
			requests = requests[:0]
			if err != nil {
				return formatError(err)
			}
		}
	}
}

func (endpoint *Endpoint) processOrders(ctx context.Context, stream pb.Orders_SettlementServer, requests []*ProcessOrderRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	responses, err := endpoint.DB.ProcessOrders(ctx, requests)
	if err != nil {
		return err
	}

	for _, response := range responses {
		err = monitoredSettlementStreamSend(ctx, stream, response)
		if err != nil {
			return err
		}
	}
	return nil
}
