// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/date"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/bandwidth"
)

var _ multinodepb.DRPCBandwidthServer = (*BandwidthEndpoint)(nil)

// BandwidthEndpoint implements multinode bandwidth endpoint.
//
// architecture: Endpoint
type BandwidthEndpoint struct {
	multinodepb.DRPCBandwidthUnimplementedServer

	log     *zap.Logger
	apiKeys *apikeys.Service
	db      bandwidth.DB
}

// NewBandwidthEndpoint creates new multinode bandwidth endpoint.
func NewBandwidthEndpoint(log *zap.Logger, apiKeys *apikeys.Service, db bandwidth.DB) *BandwidthEndpoint {
	return &BandwidthEndpoint{
		log:     log,
		apiKeys: apiKeys,
		db:      db,
	}
}

// MonthSummary returns bandwidth used current month.
func (bandwidth *BandwidthEndpoint) MonthSummary(ctx context.Context, req *multinodepb.BandwidthMonthSummaryRequest) (_ *multinodepb.BandwidthMonthSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	used, err := bandwidth.db.MonthSummary(ctx, time.Now())
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.BandwidthMonthSummaryResponse{
		Used: used,
	}, nil
}

// BandwidthSummarySatellite returns bandwidth summary for specific satellite.
func (bandwidth *BandwidthEndpoint) BandwidthSummarySatellite(ctx context.Context, req *multinodepb.BandwidthSummarySatelliteRequest) (_ *multinodepb.BandwidthSummarySatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	bandwidthSummary, err := bandwidth.db.SatelliteSummary(ctx, req.SatelliteId, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.BandwidthSummarySatelliteResponse{Summary: bandwidthSummary.Total()}, nil
}

// BandwidthSummary returns bandwidth summary.
func (bandwidth *BandwidthEndpoint) BandwidthSummary(ctx context.Context, req *multinodepb.BandwidthSummaryRequest) (_ *multinodepb.BandwidthSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	bandwidthSummary, err := bandwidth.db.Summary(ctx, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.BandwidthSummaryResponse{Summary: bandwidthSummary.Total()}, nil
}

// EgressSummarySatellite returns egress summary for specific satellite.
func (bandwidth *BandwidthEndpoint) EgressSummarySatellite(ctx context.Context, req *multinodepb.EgressSummarySatelliteRequest) (_ *multinodepb.EgressSummarySatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	egressSummary, err := bandwidth.db.SatelliteEgressSummary(ctx, req.SatelliteId, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EgressSummarySatelliteResponse{Summary: egressSummary.Total()}, nil
}

// EgressSummary returns egress summary.
func (bandwidth *BandwidthEndpoint) EgressSummary(ctx context.Context, req *multinodepb.EgressSummaryRequest) (_ *multinodepb.EgressSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	egressSummary, err := bandwidth.db.EgressSummary(ctx, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EgressSummaryResponse{Summary: egressSummary.Total()}, nil
}

// IngressSummarySatellite returns ingress summary for specific satellite.
func (bandwidth *BandwidthEndpoint) IngressSummarySatellite(ctx context.Context, req *multinodepb.IngressSummarySatelliteRequest) (_ *multinodepb.IngressSummarySatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	ingressSummary, err := bandwidth.db.SatelliteIngressSummary(ctx, req.SatelliteId, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.IngressSummarySatelliteResponse{Summary: ingressSummary.Total()}, nil
}

// IngressSummary returns ingress summary.
func (bandwidth *BandwidthEndpoint) IngressSummary(ctx context.Context, req *multinodepb.IngressSummaryRequest) (_ *multinodepb.IngressSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	ingressSummary, err := bandwidth.db.IngressSummary(ctx, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.IngressSummaryResponse{Summary: ingressSummary.Total()}, nil
}

// DailySatellite returns bandwidth summary split by days current month for specific satellite.
func (bandwidth *BandwidthEndpoint) DailySatellite(ctx context.Context, req *multinodepb.DailySatelliteRequest) (_ *multinodepb.DailySatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	bandwidthDaily, err := bandwidth.db.GetDailySatelliteRollups(ctx, req.SatelliteId, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	var resp []*multinodepb.UsageRollup
	for _, bd := range bandwidthDaily {
		resp = append(resp, &multinodepb.UsageRollup{
			Egress: &multinodepb.Egress{
				Repair: bd.Egress.Repair,
				Audit:  bd.Egress.Audit,
				Usage:  bd.Egress.Usage,
			},
			Ingress: &multinodepb.Ingress{
				Repaid: bd.Ingress.Repair,
				Usage:  bd.Ingress.Usage,
			},
			Delete:        bd.Delete,
			IntervalStart: bd.IntervalStart,
		})
	}

	return &multinodepb.DailySatelliteResponse{UsageRollup: resp}, nil
}

// Daily returns bandwidth summary split by days current month.
func (bandwidth *BandwidthEndpoint) Daily(ctx context.Context, req *multinodepb.DailyRequest) (_ *multinodepb.DailyResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	from, to := date.MonthBoundary(time.Now().UTC())
	bandwidthDaily, err := bandwidth.db.GetDailyRollups(ctx, from, to)
	if err != nil {
		bandwidth.log.Error("bandwidth internal error", zap.Error(err))
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	var resp []*multinodepb.UsageRollup
	for _, bd := range bandwidthDaily {
		resp = append(resp, &multinodepb.UsageRollup{
			Egress: &multinodepb.Egress{
				Repair: bd.Egress.Repair,
				Audit:  bd.Egress.Audit,
				Usage:  bd.Egress.Usage,
			},
			Ingress: &multinodepb.Ingress{
				Repaid: bd.Ingress.Repair,
				Usage:  bd.Ingress.Usage,
			},
			Delete:        bd.Delete,
			IntervalStart: bd.IntervalStart,
		})
	}

	return &multinodepb.DailyResponse{UsageRollup: resp}, nil
}
