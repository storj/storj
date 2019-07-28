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
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, certdb certdb.DB, db DB) *Endpoint {
	return &Endpoint{
		log:             log,
		satelliteSignee: satelliteSignee,
		DB:              db,
		certdb:          certdb,
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
	for {
		// TODO: batch these requests so we hit the db in batches
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

		bucketID, err := endpoint.DB.UseSerialNumber(ctx, orderLimit.SerialNumber, orderLimit.StorageNodeId)
		if err != nil {
			log.Warn("unable to use serial number", zap.Error(err))
			if ErrUsingSerialNumber.Has(err) {
				err := monitoredSettlementStreamSend(ctx, stream, &pb.SettlementResponse{
					SerialNumber: orderLimit.SerialNumber,
					Status:       pb.SettlementResponse_REJECTED,
				})
				if err != nil {
					return formatError(err)
				}
				continue
			}
			return err
		}
		now := time.Now().UTC()
		intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

		projectID, bucketName, err := SplitBucketID(bucketID)
		if err != nil {
			return err
		}
		if err := endpoint.DB.UpdateBucketBandwidthSettle(ctx, *projectID, bucketName, orderLimit.Action, order.Amount, intervalStart); err != nil {
			// TODO: we should use the serial number in the same transaction we settle the bandwidth? that way we don't need this undo in an error case?
			if err := endpoint.DB.UnuseSerialNumber(ctx, orderLimit.SerialNumber, orderLimit.StorageNodeId); err != nil {
				log.Error("unable to unuse serial number", zap.Error(err))
			}
			return err
		}

		// TODO: whoa this should also be in the same transaction
		if err := endpoint.DB.UpdateStoragenodeBandwidthSettle(ctx, orderLimit.StorageNodeId, orderLimit.Action, order.Amount, intervalStart); err != nil {
			if err := endpoint.DB.UnuseSerialNumber(ctx, orderLimit.SerialNumber, orderLimit.StorageNodeId); err != nil {
				log.Error("unable to unuse serial number", zap.Error(err))
			}
			if err := endpoint.DB.UpdateBucketBandwidthSettle(ctx, *projectID, bucketName, orderLimit.Action, -order.Amount, intervalStart); err != nil {
				log.Error("unable to rollback bucket bandwidth", zap.Error(err))
			}
			return err
		}

		err = monitoredSettlementStreamSend(ctx, stream, &pb.SettlementResponse{
			SerialNumber: orderLimit.SerialNumber,
			Status:       pb.SettlementResponse_ACCEPTED,
		})
		if err != nil {
			return formatError(err)
		}

		// TODO: in fact, why don't we batch these into group transactions as they come in from Recv() ?
	}
}
