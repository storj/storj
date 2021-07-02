// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
)

var _ multinodepb.DRPCPayoutServer = (*PayoutEndpoint)(nil)
var _ multinodepb.DRPCPayoutsServer = (*PayoutEndpoint)(nil)

// PayoutEndpoint implements multinode payouts endpoint.
//
// architecture: Endpoint
type PayoutEndpoint struct {
	multinodepb.DRPCPayoutUnimplementedServer
	multinodepb.DRPCPayoutsUnimplementedServer

	log              *zap.Logger
	apiKeys          *apikeys.Service
	db               payouts.DB
	service          *payouts.Service
	estimatedPayouts *estimatedpayouts.Service
}

// NewPayoutEndpoint creates new multinode payouts endpoint.
func NewPayoutEndpoint(log *zap.Logger, apiKeys *apikeys.Service, db payouts.DB, estimatedPayouts *estimatedpayouts.Service, service *payouts.Service) *PayoutEndpoint {
	return &PayoutEndpoint{
		log:              log,
		apiKeys:          apiKeys,
		db:               db,
		service:          service,
		estimatedPayouts: estimatedPayouts,
	}
}

// Earned returns total earned amount.
func (payout *PayoutEndpoint) Earned(ctx context.Context, req *multinodepb.EarnedRequest) (_ *multinodepb.EarnedResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	earned, err := payout.db.GetTotalEarned(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EarnedResponse{
		Total: earned,
	}, nil
}

// EarnedSatellite returns total earned amount per satellite.
func (payout *PayoutEndpoint) EarnedSatellite(ctx context.Context, req *multinodepb.EarnedSatelliteRequest) (_ *multinodepb.EarnedSatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var resp multinodepb.EarnedSatelliteResponse

	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for _, id := range satelliteIDs {
		earned, err := payout.db.GetEarnedAtSatellite(ctx, id)
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		resp.EarnedSatellite = append(resp.EarnedSatellite, &multinodepb.EarnedSatellite{
			Total:       earned,
			SatelliteId: id,
		})
	}

	return &resp, nil
}

// EstimatedPayout returns estimated earnings for current month from all satellites.
func (payout *PayoutEndpoint) EstimatedPayout(ctx context.Context, req *multinodepb.EstimatedPayoutRequest) (_ *multinodepb.EstimatedPayoutResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	estimated, err := payout.estimatedPayouts.GetAllSatellitesEstimatedPayout(ctx, time.Now())
	if err != nil {
		return &multinodepb.EstimatedPayoutResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EstimatedPayoutResponse{EstimatedEarnings: estimated.CurrentMonthExpectations}, nil
}

// EstimatedPayoutSatellite returns estimated earnings for current month from specific satellite.
func (payout *PayoutEndpoint) EstimatedPayoutSatellite(ctx context.Context, req *multinodepb.EstimatedPayoutSatelliteRequest) (_ *multinodepb.EstimatedPayoutSatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	estimated, err := payout.estimatedPayouts.GetSatelliteEstimatedPayout(ctx, req.SatelliteId, time.Now())
	if err != nil {
		return &multinodepb.EstimatedPayoutSatelliteResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EstimatedPayoutSatelliteResponse{EstimatedEarnings: estimated.CurrentMonthExpectations}, nil
}

// Summary returns all satellites all time payout summary.
func (payout *PayoutEndpoint) Summary(ctx context.Context, req *multinodepb.SummaryRequest) (_ *multinodepb.SummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64

	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return &multinodepb.SummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for _, id := range satelliteIDs {
		paid, held, err := payout.db.GetSatelliteSummary(ctx, id)
		if err != nil {
			return &multinodepb.SummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		totalHeld += held
		totalPaid += paid
	}

	return &multinodepb.SummaryResponse{PayoutInfo: &multinodepb.PayoutInfo{Paid: totalPaid, Held: totalHeld}}, nil
}

// SummaryPeriod returns all satellites period payout summary.
func (payout *PayoutEndpoint) SummaryPeriod(ctx context.Context, req *multinodepb.SummaryPeriodRequest) (_ *multinodepb.SummaryPeriodResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64

	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return &multinodepb.SummaryPeriodResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for _, id := range satelliteIDs {
		paid, held, err := payout.db.GetSatellitePeriodSummary(ctx, id, req.Period)
		if err != nil {
			return &multinodepb.SummaryPeriodResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		totalHeld += held
		totalPaid += paid
	}

	return &multinodepb.SummaryPeriodResponse{PayoutInfo: &multinodepb.PayoutInfo{Held: totalHeld, Paid: totalPaid}}, nil
}

// SummarySatellite returns satellite all time payout summary.
func (payout *PayoutEndpoint) SummarySatellite(ctx context.Context, req *multinodepb.SummarySatelliteRequest) (_ *multinodepb.SummarySatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64

	totalPaid, totalHeld, err = payout.db.GetSatelliteSummary(ctx, req.SatelliteId)
	if err != nil {
		return &multinodepb.SummarySatelliteResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.SummarySatelliteResponse{PayoutInfo: &multinodepb.PayoutInfo{Held: totalHeld, Paid: totalPaid}}, nil
}

// SummarySatellitePeriod returns satellite period payout summary.
func (payout *PayoutEndpoint) SummarySatellitePeriod(ctx context.Context, req *multinodepb.SummarySatellitePeriodRequest) (_ *multinodepb.SummarySatellitePeriodResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64

	totalPaid, totalHeld, err = payout.db.GetSatellitePeriodSummary(ctx, req.SatelliteId, req.Period)
	if err != nil {
		return &multinodepb.SummarySatellitePeriodResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.SummarySatellitePeriodResponse{PayoutInfo: &multinodepb.PayoutInfo{Held: totalHeld, Paid: totalPaid}}, nil
}

// Undistributed returns total undistributed amount.
func (payout *PayoutEndpoint) Undistributed(ctx context.Context, req *multinodepb.UndistributedRequest) (_ *multinodepb.UndistributedResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	earned, err := payout.db.GetUndistributed(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.UndistributedResponse{Total: earned}, nil
}

// PaystubSatellite returns summed amounts of all values from paystubs from all satellites.
func (payout *PayoutEndpoint) PaystubSatellite(ctx context.Context, req *multinodepb.PaystubSatelliteRequest) (_ *multinodepb.PaystubSatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetSatellitePaystubs(ctx, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.PaystubSatelliteResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}

// Paystub returns summed amounts of all values from paystubs from all satellites.
func (payout *PayoutEndpoint) Paystub(ctx context.Context, req *multinodepb.PaystubRequest) (_ *multinodepb.PaystubResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetPaystubs(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.PaystubResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}

// PaystubPeriod returns summed amounts of all values from paystubs from all satellites for specific period.
func (payout *PayoutEndpoint) PaystubPeriod(ctx context.Context, req *multinodepb.PaystubPeriodRequest) (_ *multinodepb.PaystubPeriodResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetPeriodPaystubs(ctx, req.Period)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.PaystubPeriodResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}

// PaystubSatellitePeriod returns summed amounts of all values from paystubs from all satellites for specific period.
func (payout *PayoutEndpoint) PaystubSatellitePeriod(ctx context.Context, req *multinodepb.PaystubSatellitePeriodRequest) (_ *multinodepb.PaystubSatellitePeriodResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetSatellitePeriodPaystubs(ctx, req.Period, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.PaystubSatellitePeriodResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}

// HeldAmountHistory returns held amount history for all satellites.
func (payout *PayoutEndpoint) HeldAmountHistory(ctx context.Context, req *multinodepb.HeldAmountHistoryRequest) (_ *multinodepb.HeldAmountHistoryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	heldHistory, err := payout.service.HeldAmountHistory(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	resp := new(multinodepb.HeldAmountHistoryResponse)

	for _, satelliteHeldHistory := range heldHistory {
		var pbHeldAmount []*multinodepb.HeldAmountHistoryResponse_HeldAmount

		for _, heldAmount := range satelliteHeldHistory.HeldAmounts {
			pbHeldAmount = append(pbHeldAmount, &multinodepb.HeldAmountHistoryResponse_HeldAmount{
				Period: heldAmount.Period,
				Amount: heldAmount.Amount,
			})
		}

		resp.History = append(resp.History, &multinodepb.HeldAmountHistoryResponse_HeldAmountHistory{
			SatelliteId: satelliteHeldHistory.SatelliteID,
			HeldAmounts: pbHeldAmount,
		})
	}

	return resp, nil
}

// PeriodPaystub returns summed amounts of all values from paystubs from all satellites for specific period.
func (payout *PayoutEndpoint) PeriodPaystub(ctx context.Context, req *multinodepb.PeriodPaystubRequest) (_ *multinodepb.PeriodPaystubResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetPeriodPaystubs(ctx, req.Period)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.PeriodPaystubResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}

// EarnedPerSatellite returns total earned amount per satellite.
func (payout *PayoutEndpoint) EarnedPerSatellite(ctx context.Context, req *multinodepb.EarnedPerSatelliteRequest) (_ *multinodepb.EarnedPerSatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var resp multinodepb.EarnedPerSatelliteResponse
	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for _, id := range satelliteIDs {
		earned, err := payout.db.GetEarnedAtSatellite(ctx, id)
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		resp.EarnedSatellite = append(resp.EarnedSatellite, &multinodepb.EarnedSatellite{
			Total:       earned,
			SatelliteId: id,
		})
	}

	return &resp, nil
}

// EstimatedPayoutTotal returns estimated earnings for current month from all satellites.
func (payout *PayoutEndpoint) EstimatedPayoutTotal(ctx context.Context, req *multinodepb.EstimatedPayoutTotalRequest) (_ *multinodepb.EstimatedPayoutTotalResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	estimated, err := payout.estimatedPayouts.GetAllSatellitesEstimatedPayout(ctx, time.Now())
	if err != nil {
		return &multinodepb.EstimatedPayoutTotalResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EstimatedPayoutTotalResponse{EstimatedEarnings: estimated.CurrentMonthExpectations}, nil
}

// AllSatellitesSummary returns all satellites all time payout summary.
func (payout *PayoutEndpoint) AllSatellitesSummary(ctx context.Context, req *multinodepb.AllSatellitesSummaryRequest) (_ *multinodepb.AllSatellitesSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64
	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return &multinodepb.AllSatellitesSummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for _, id := range satelliteIDs {
		paid, held, err := payout.db.GetSatelliteSummary(ctx, id)
		if err != nil {
			return &multinodepb.AllSatellitesSummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		totalHeld += held
		totalPaid += paid
	}

	return &multinodepb.AllSatellitesSummaryResponse{PayoutInfo: &multinodepb.PayoutInfo{Paid: totalPaid, Held: totalHeld}}, nil
}

// AllSatellitesPeriodSummary returns all satellites period payout summary.
func (payout *PayoutEndpoint) AllSatellitesPeriodSummary(ctx context.Context, req *multinodepb.AllSatellitesPeriodSummaryRequest) (_ *multinodepb.AllSatellitesPeriodSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64
	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return &multinodepb.AllSatellitesPeriodSummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for _, id := range satelliteIDs {
		paid, held, err := payout.db.GetSatellitePeriodSummary(ctx, id, req.Period)
		if err != nil {
			return &multinodepb.AllSatellitesPeriodSummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		totalHeld += held
		totalPaid += paid
	}

	return &multinodepb.AllSatellitesPeriodSummaryResponse{PayoutInfo: &multinodepb.PayoutInfo{Held: totalHeld, Paid: totalPaid}}, nil
}

// SatelliteSummary returns satellite all time payout summary.
func (payout *PayoutEndpoint) SatelliteSummary(ctx context.Context, req *multinodepb.SatelliteSummaryRequest) (_ *multinodepb.SatelliteSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64

	totalPaid, totalHeld, err = payout.db.GetSatelliteSummary(ctx, req.SatelliteId)
	if err != nil {
		return &multinodepb.SatelliteSummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.SatelliteSummaryResponse{PayoutInfo: &multinodepb.PayoutInfo{Held: totalHeld, Paid: totalPaid}}, nil
}

// SatellitePeriodSummary returns satellite period payout summary.
func (payout *PayoutEndpoint) SatellitePeriodSummary(ctx context.Context, req *multinodepb.SatellitePeriodSummaryRequest) (_ *multinodepb.SatellitePeriodSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var totalPaid, totalHeld int64

	totalPaid, totalHeld, err = payout.db.GetSatellitePeriodSummary(ctx, req.SatelliteId, req.Period)
	if err != nil {
		return &multinodepb.SatellitePeriodSummaryResponse{}, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.SatellitePeriodSummaryResponse{PayoutInfo: &multinodepb.PayoutInfo{Held: totalHeld, Paid: totalPaid}}, nil
}

// SatellitePaystub returns summed amounts of all values from paystubs from all satellites.
func (payout *PayoutEndpoint) SatellitePaystub(ctx context.Context, req *multinodepb.SatellitePaystubRequest) (_ *multinodepb.SatellitePaystubResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetSatellitePaystubs(ctx, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.SatellitePaystubResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}

// SatellitePeriodPaystub returns summed amounts of all values from paystubs from all satellites for specific period.
func (payout *PayoutEndpoint) SatellitePeriodPaystub(ctx context.Context, req *multinodepb.SatellitePeriodPaystubRequest) (_ *multinodepb.SatellitePeriodPaystubResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	paystub, err := payout.db.GetSatellitePeriodPaystubs(ctx, req.Period, req.SatelliteId)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.SatellitePeriodPaystubResponse{Paystub: &multinodepb.Paystub{
		UsageAtRest:    paystub.UsageAtRest,
		UsageGet:       paystub.UsageGet,
		UsageGetRepair: paystub.UsageGetRepair,
		UsageGetAudit:  paystub.UsageGetAudit,
		CompAtRest:     paystub.CompAtRest,
		CompGet:        paystub.CompGet,
		CompGetRepair:  paystub.CompGetRepair,
		CompGetAudit:   paystub.CompGetAudit,
		Held:           paystub.Held,
		Paid:           paystub.Paid,
		Distributed:    paystub.Distributed,
		Disposed:       paystub.Disposed,
	}}, nil
}
