// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"strings"
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
)

// DB implements saving order after receiving from storage node
type DB interface {
	// SaveInlineOrder
	SaveInlineOrder(ctx context.Context, bucketID []byte) error
	// SaveRemoteOrder
	SaveRemoteOrder(ctx context.Context, bucketID []byte, orderLimits []*pb.OrderLimit2) error
	// SettleOrder
	SettleRemoteOrder(ctx context.Context, orderLimit *pb.OrderLimit2, order *pb.Order2) error
}

var (
	// Error the default orders errs class
	Error = errs.Class("orders error")
	mon   = monkit.Package()
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

		uplinkPubKey, err := endpoint.certdb.GetPublicKey(ctx, orderLimit.UplinkId)
		if err != nil {
			endpoint.log.Warn("unable to find uplink public key", zap.Error(err))
			return status.Errorf(codes.Internal, "unable to find uplink public key")
		}

		rejectErr := func() error {
			if err := signing.VerifyOrderLimitSignature(endpoint.satelliteSignee, orderLimit); err != nil {
				return Error.New("unable to verify order limit")
			}

			uplinkSignee := &signing.PublicKey{
				Self: orderLimit.UplinkId, // TODO should this be taken from public key ??
				Key:  uplinkPubKey,
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

		if err = endpoint.DB.SettleRemoteOrder(ctx, orderLimit, order); err != nil {
			duplicateRequest := strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "violates unique constraint")
			if duplicateRequest {
				err := stream.Send(&pb.SettlementResponse{
					SerialNumber: orderLimit.SerialNumber,
					Status:       pb.SettlementResponse_REJECTED,
				})
				if err != nil {
					return formatError(err)
				}
			} else {
				// send error if order was not saved to DB to avoid removing on storage node
				return err
			}
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
