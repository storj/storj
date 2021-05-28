// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package snopayouts

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/date"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()
)

// Endpoint for querying node stats for the SNO.
//
// architecture: Endpoint
type Endpoint struct {
	pb.DRPCHeldAmountUnimplementedServer

	service    *Service
	log        *zap.Logger
	overlay    overlay.DB
	accounting accounting.StoragenodeAccounting
}

// NewEndpoint creates new endpoint.
func NewEndpoint(log *zap.Logger, accounting accounting.StoragenodeAccounting, overlay overlay.DB, service *Service) *Endpoint {
	return &Endpoint{
		log:        log,
		accounting: accounting,
		overlay:    overlay,
		service:    service,
	}
}

// GetPayStub sends node paystub for client node.
func (e *Endpoint) GetPayStub(ctx context.Context, req *pb.GetHeldAmountRequest) (_ *pb.GetHeldAmountResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	paystub, err := e.service.GetPaystub(ctx, node.Id, req.Period.Format("2006-01"))
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Wrap(rpcstatus.OutOfRange, err)
		}
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return convertPaystub(paystub)
}

// GetAllPaystubs sends all paystubs for client node.
func (e *Endpoint) GetAllPaystubs(ctx context.Context, req *pb.GetAllPaystubsRequest) (_ *pb.GetAllPaystubsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return &pb.GetAllPaystubsResponse{}, nil
		}
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	paystubs, err := e.service.GetAllPaystubs(ctx, node.Id)
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Wrap(rpcstatus.OutOfRange, err)
		}
		return nil, Error.Wrap(err)
	}

	response := &pb.GetAllPaystubsResponse{}
	for _, paystub := range paystubs {
		pbPaystub, err := convertPaystub(paystub)
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
		}
		response.Paystub = append(response.Paystub, pbPaystub)
	}
	return response, nil
}

func convertPaystub(paystub Paystub) (*pb.GetHeldAmountResponse, error) {
	period, err := date.PeriodToTime(paystub.Period)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, Error.Wrap(err))
	}

	return &pb.GetHeldAmountResponse{
		Period:         period,
		NodeId:         paystub.NodeID,
		CreatedAt:      paystub.Created,
		Codes:          paystub.Codes,
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsagePut:       paystub.UsagePut,
		UsageGetRepair: paystub.UsageGetRepair,
		UsagePutRepair: paystub.UsagePutRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompPut:        paystub.CompPut,
		CompGetRepair:  paystub.CompGetRepair,
		CompPutRepair:  paystub.CompPutRepair,
		CompGetAudit:   paystub.CompGetAudit,
		SurgePercent:   paystub.SurgePercent,
		Held:           paystub.Held,
		Owed:           paystub.Owed,
		Disposed:       paystub.Disposed,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
	}, err
}

// GetPayment sends node payment data for client node.
func (e *Endpoint) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (_ *pb.GetPaymentResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	payment, err := e.service.GetPayment(ctx, node.Id, req.Period.String())
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Wrap(rpcstatus.OutOfRange, err)
		}
		return nil, Error.Wrap(err)
	}

	return convertPayment(payment)
}

// GetAllPayments sends all payments to node.
func (e *Endpoint) GetAllPayments(ctx context.Context, req *pb.GetAllPaymentsRequest) (_ *pb.GetAllPaymentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return &pb.GetAllPaymentsResponse{}, nil
		}
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	payments, err := e.service.GetAllPayments(ctx, node.Id)
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Wrap(rpcstatus.OutOfRange, err)
		}
		return nil, Error.Wrap(err)
	}

	response := &pb.GetAllPaymentsResponse{}
	for _, payment := range payments {
		pbPayment, err := convertPayment(payment)
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
		}
		response.Payment = append(response.Payment, pbPayment)
	}
	return response, nil
}

func convertPayment(payment Payment) (*pb.GetPaymentResponse, error) {
	period, err := date.PeriodToTime(payment.Period)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, Error.Wrap(err))
	}

	return &pb.GetPaymentResponse{
		Id:        payment.ID,
		CreatedAt: payment.Created,
		NodeId:    payment.NodeID,
		Period:    period,
		Amount:    payment.Amount,
		Receipt:   payment.Receipt,
		Notes:     payment.Notes,
	}, nil
}
