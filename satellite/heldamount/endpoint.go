// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package heldamount

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

// NewEndpoint creates new endpoint
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
		if overlay.ErrNodeNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
		}
		e.log.Error("overlay.Get failed", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	period := req.Period.String()[0:7]
	stub, err := e.service.GetPayStub(ctx, node.Id, period)
	if err != nil {
		return nil, err
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
		UsageAtRest:    float32(stub.UsageAtRest),
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

// GetPayment sends node payment data for client node.
func (e *Endpoint) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (_ *pb.GetPaymentResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
		}
		e.log.Error("overlay.Get failed", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	payment, err := e.service.GetPayment(ctx, node.Id, req.Period.String())
	if err != nil {
		return nil, err
	}

	timePeriod, err := date.PeriodToTime(payment.Period)
	if err != nil {
		return nil, err
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
