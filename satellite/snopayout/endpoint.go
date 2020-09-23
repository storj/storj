// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package snopayout

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

// Endpoint for querying node stats for the SNO
//
// architecture: Endpoint
type Endpoint struct {
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
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	period := req.Period.String()[0:7]
	stub, err := e.service.GetPayStub(ctx, node.Id, period)
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.OutOfRange, err.Error())
		}
		return nil, Error.Wrap(err)
	}

	periodTime, err := date.PeriodToTime(stub.Period)
	if err != nil {
		return nil, err
	}
	return &pb.GetHeldAmountResponse{
		Period:         periodTime,
		NodeId:         stub.NodeID,
		CreatedAt:      stub.Created,
		Codes:          stub.Codes,
		UsageAtRest:    stub.UsageAtRest,
		UsageGet:       stub.UsageGet,
		UsagePut:       stub.UsagePut,
		UsageGetRepair: stub.UsageGetRepair,
		UsagePutRepair: stub.UsagePutRepair,
		UsageGetAudit:  stub.UsageGetAudit,
		CompAtRest:     stub.CompAtRest,
		CompGet:        stub.CompGet,
		CompPut:        stub.CompPut,
		CompGetRepair:  stub.CompGetRepair,
		CompPutRepair:  stub.CompPutRepair,
		CompGetAudit:   stub.CompGetAudit,
		SurgePercent:   stub.SurgePercent,
		Held:           stub.Held,
		Owed:           stub.Owed,
		Disposed:       stub.Disposed,
		Paid:           stub.Paid,
	}, nil
}

// GetAllPaystubs sends all paystubs for client node.
func (e *Endpoint) GetAllPaystubs(ctx context.Context, req *pb.GetAllPaystubsRequest) (_ *pb.GetAllPaystubsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return &pb.GetAllPaystubsResponse{}, nil
		}

		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	stubs, err := e.service.GetAllPaystubs(ctx, node.Id)
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.OutOfRange, err.Error())
		}
		return nil, Error.Wrap(err)
	}

	var paystubs []*pb.GetHeldAmountResponse

	response := pb.GetAllPaystubsResponse{
		Paystub: paystubs,
	}

	for i := 0; i < len(stubs); i++ {
		period, err := date.PeriodToTime(stubs[i].Period)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		heldAmountResponse := pb.GetHeldAmountResponse{
			Period:         period,
			NodeId:         stubs[i].NodeID,
			CreatedAt:      stubs[i].Created,
			Codes:          stubs[i].Codes,
			UsageAtRest:    stubs[i].UsageAtRest,
			UsageGet:       stubs[i].UsageGet,
			UsagePut:       stubs[i].UsagePut,
			UsageGetRepair: stubs[i].UsageGetRepair,
			UsagePutRepair: stubs[i].UsagePutRepair,
			UsageGetAudit:  stubs[i].UsageGetAudit,
			CompAtRest:     stubs[i].CompAtRest,
			CompGet:        stubs[i].CompGet,
			CompPut:        stubs[i].CompPut,
			CompGetRepair:  stubs[i].CompGetRepair,
			CompPutRepair:  stubs[i].CompPutRepair,
			CompGetAudit:   stubs[i].CompGetAudit,
			SurgePercent:   stubs[i].SurgePercent,
			Held:           stubs[i].Held,
			Owed:           stubs[i].Owed,
			Disposed:       stubs[i].Disposed,
			Paid:           stubs[i].Paid,
		}

		response.Paystub = append(response.Paystub, &heldAmountResponse)
	}

	return &response, nil
}

// GetPayment sends node payment data for client node.
func (e *Endpoint) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (_ *pb.GetPaymentResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	payment, err := e.service.GetPayment(ctx, node.Id, req.Period.String())
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.OutOfRange, err.Error())
		}
		return nil, Error.Wrap(err)
	}

	timePeriod, err := date.PeriodToTime(payment.Period)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &pb.GetPaymentResponse{
		NodeId:    payment.NodeID,
		CreatedAt: payment.Created,
		Period:    timePeriod,
		Amount:    payment.Amount,
		Receipt:   payment.Receipt,
		Notes:     payment.Notes,
		Id:        payment.ID,
	}, nil
}

// GetAllPayments sends all payments to node.
func (e *Endpoint) GetAllPayments(ctx context.Context, req *pb.GetAllPaymentsRequest) (_ *pb.GetAllPaymentsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return &pb.GetAllPaymentsResponse{}, nil
		}

		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	allPayments, err := e.service.GetAllPayments(ctx, node.Id)
	if err != nil {
		if ErrNoDataForPeriod.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.OutOfRange, err.Error())
		}
		return nil, Error.Wrap(err)
	}

	var payments []*pb.GetPaymentResponse

	response := pb.GetAllPaymentsResponse{
		Payment: payments,
	}

	for i := 0; i < len(allPayments); i++ {
		period, err := date.PeriodToTime(allPayments[i].Period)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		paymentResponse := pb.GetPaymentResponse{
			NodeId:    allPayments[i].NodeID,
			CreatedAt: allPayments[i].Created,
			Period:    period,
			Amount:    allPayments[i].Amount,
			Receipt:   allPayments[i].Receipt,
			Notes:     allPayments[i].Notes,
			Id:        allPayments[i].ID,
		}

		response.Payment = append(response.Payment, &paymentResponse)
	}

	return &response, nil
}
