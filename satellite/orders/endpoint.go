// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/certdb"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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
	UpdateBucketBandwidthAllocation(ctx context.Context, bucketID []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
	UpdateBucketBandwidthSettle(ctx context.Context, bucketID []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
	UpdateBucketBandwidthInline(ctx context.Context, bucketID []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// UpdateStoragenodeBandwidthAllocation updates 'allocated' bandwidth for given storage node
	UpdateStoragenodeBandwidthAllocation(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// GetBucketBandwidth gets total bucket bandwidth from period of time
	GetBucketBandwidth(ctx context.Context, bucketID []byte, from, to time.Time) (int64, error)
	// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
	GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error)
}

var (
	// Error the default orders errs class
	Error = errs.Class("orders error")
	// ErrUsingSerialNumber error class for serial number
	ErrUsingSerialNumber = errs.Class("serial number")

	mon = monkit.Package()
)

// Endpoint for orders receiving
type Endpoint struct {
	log             *zap.Logger
	satelliteSignee signing.Signee
	DB              DB
	certdb          certdb.DB
}

// NewEndpoint new orders receiving endpoint
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, db DB, certdb certdb.DB) *Endpoint {
	return &Endpoint{
		log:             log,
		satelliteSignee: satelliteSignee,
		DB:              db,
		certdb:          certdb,
	}
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

	endpoint.log.Debug("Settlement", zap.Any("storage node ID", peer.ID))
	for {
		request, err := stream.Recv()
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

		orderExpiration, err := ptypes.Timestamp(orderLimit.OrderExpiration)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, err.Error())
		}

		var uplinkSignee signing.Signee

		// who asked for this order: uplink (get/put/del) or satellite (get_repair/put_repair/audit)
		if endpoint.satelliteSignee.ID() == orderLimit.UplinkId {
			uplinkSignee = endpoint.satelliteSignee
		} else {
			uplinkPubKey, err := endpoint.certdb.GetPublicKey(ctx, orderLimit.UplinkId)
			if err != nil {
				endpoint.log.Warn("unable to find uplink public key", zap.Error(err))
				return status.Errorf(codes.Internal, "unable to find uplink public key")
			}
			uplinkSignee = &signing.PublicKey{
				Self: orderLimit.UplinkId,
				Key:  uplinkPubKey,
			}
		}

		rejectErr := func() error {
			if err := signing.VerifyOrderLimitSignature(endpoint.satelliteSignee, orderLimit); err != nil {
				return Error.New("unable to verify order limit")
			}

			if err := signing.VerifyOrderSignature(uplinkSignee, order); err != nil {
				return Error.New("unable to verify order")
			}

			// TODO should this reject or just error ??
			if orderLimit.SerialNumber != order.SerialNumber {
				return Error.New("invalid serial number")
			}

			if orderExpiration.Before(time.Now()) {
				return Error.New("order limit expired")
			}
			return nil
		}()
		if rejectErr != err {
			endpoint.log.Debug("order limit/order verification failed", zap.String("serial", orderLimit.SerialNumber.String()), zap.Error(err))
			err := stream.Send(&pb.SettlementResponse{
				SerialNumber: orderLimit.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			if err != nil {
				return formatError(err)
			}
		}

		bucketID, err := endpoint.DB.UseSerialNumber(ctx, orderLimit.SerialNumber, orderLimit.StorageNodeId)
		if err != nil {
			endpoint.log.Warn("unable to use serial number", zap.Error(err))
			if ErrUsingSerialNumber.Has(err) {
				err := stream.Send(&pb.SettlementResponse{
					SerialNumber: orderLimit.SerialNumber,
					Status:       pb.SettlementResponse_REJECTED,
				})
				if err != nil {
					return formatError(err)
				}
			} else {
				return err
			}
			continue
		}
		now := time.Now()
		intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

		if err := endpoint.DB.UpdateBucketBandwidthSettle(ctx, bucketID, orderLimit.Action, order.Amount, intervalStart); err != nil {
			if err := endpoint.DB.UnuseSerialNumber(ctx, orderLimit.SerialNumber, orderLimit.StorageNodeId); err != nil {
				endpoint.log.Error("unable to unuse serial number", zap.Error(err))
			}
			return err
		}

		if err := endpoint.DB.UpdateStoragenodeBandwidthSettle(ctx, orderLimit.StorageNodeId, orderLimit.Action, order.Amount, intervalStart); err != nil {
			if err := endpoint.DB.UnuseSerialNumber(ctx, orderLimit.SerialNumber, orderLimit.StorageNodeId); err != nil {
				endpoint.log.Error("unable to unuse serial number", zap.Error(err))
			}
			if err := endpoint.DB.UpdateBucketBandwidthSettle(ctx, bucketID, orderLimit.Action, -order.Amount, intervalStart); err != nil {
				endpoint.log.Error("unable to rollback bucket bandwidth", zap.Error(err))
			}
			return err
		}

		err = stream.Send(&pb.SettlementResponse{
			SerialNumber: orderLimit.SerialNumber,
			Status:       pb.SettlementResponse_ACCEPTED,
		})
		if err != nil {
			return formatError(err)
		}
	}
}
