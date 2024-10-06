// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodepb"
)

var (
	mon = monkit.Package()

	// Error is an error class for bandwidth service error.
	Error = errs.Class("bandwidth")
)

// Service exposes bandwidth related logic.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
	nodes  *nodes.Service
}

// NewService creates new instance of Service.
func NewService(log *zap.Logger, dialer rpc.Dialer, nodes *nodes.Service) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		nodes:  nodes,
	}
}

// Monthly returns monthly bandwidth summary.
func (service *Service) Monthly(ctx context.Context) (_ Monthly, err error) {
	defer mon.Task()(&ctx)(&err)
	var totalMonthly Monthly

	listNodes, err := service.nodes.List(ctx)
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	cache := make(UsageRollupDailyCache)

	for _, node := range listNodes {

		monthly, err := service.getMonthly(ctx, node)
		if err != nil {
			service.log.Error("Failed to fetch the monthly bandwidth summary of the node:", zap.Error(err))
			continue
		}
		totalMonthly.IngressSummary += monthly.IngressSummary
		totalMonthly.EgressSummary += monthly.EgressSummary
		totalMonthly.BandwidthSummary += monthly.BandwidthSummary

		for _, rollup := range monthly.BandwidthDaily {
			cache.Add(rollup)
		}
	}
	totalMonthly.BandwidthDaily = cache.Sorted()

	return totalMonthly, nil
}

// MonthlyNode returns monthly bandwidth summary for single node.
func (service *Service) MonthlyNode(ctx context.Context, nodeID storj.NodeID) (_ Monthly, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	monthly, err := service.getMonthly(ctx, node)
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	return monthly, nil
}

// MonthlySatellite returns monthly bandwidth summary for specific satellite.
func (service *Service) MonthlySatellite(ctx context.Context, satelliteID storj.NodeID) (_ Monthly, err error) {
	defer mon.Task()(&ctx)(&err)
	var totalMonthly Monthly

	listNodes, err := service.nodes.List(ctx)
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	cache := make(UsageRollupDailyCache)

	for _, node := range listNodes {

		monthly, err := service.getMonthlySatellite(ctx, node, satelliteID)
		if err != nil {
			service.log.Error("Failed to fetch monthly bandwidth summary for the node and specific satellite", zap.Error(err))
			continue
		}

		totalMonthly.IngressSummary += monthly.IngressSummary
		totalMonthly.EgressSummary += monthly.EgressSummary
		totalMonthly.BandwidthSummary += monthly.BandwidthSummary

		for _, rollup := range monthly.BandwidthDaily {
			cache.Add(rollup)
		}
	}
	totalMonthly.BandwidthDaily = cache.Sorted()

	return totalMonthly, nil
}

// MonthlySatelliteNode returns monthly bandwidth summary for single node and specific satellites.
func (service *Service) MonthlySatelliteNode(ctx context.Context, satelliteID, nodeID storj.NodeID) (_ Monthly, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	monthly, err := service.getMonthlySatellite(ctx, node, satelliteID)
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	return monthly, nil
}

// getMonthlySatellite returns monthly bandwidth summary for single node and specific satellite.
func (service *Service) getMonthlySatellite(ctx context.Context, node nodes.Node, satelliteID storj.NodeID) (_ Monthly, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Monthly{}, nodes.ErrNodeNotReachable.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	bandwidthClient := multinodepb.NewDRPCBandwidthClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret[:],
	}

	ingress, err := bandwidthClient.IngressSummarySatellite(ctx, &multinodepb.IngressSummarySatelliteRequest{
		Header:      header,
		SatelliteId: satelliteID,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	egress, err := bandwidthClient.EgressSummarySatellite(ctx, &multinodepb.EgressSummarySatelliteRequest{
		Header:      header,
		SatelliteId: satelliteID,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	bandwidth, err := bandwidthClient.BandwidthSummarySatellite(ctx, &multinodepb.BandwidthSummarySatelliteRequest{
		Header:      header,
		SatelliteId: satelliteID,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	usageRollup, err := bandwidthClient.DailySatellite(ctx, &multinodepb.DailySatelliteRequest{
		Header:      header,
		SatelliteId: satelliteID,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	var rollups []UsageRollup
	for _, r := range usageRollup.UsageRollup {
		rollups = append(rollups, UsageRollup{
			Egress: Egress{
				Repair: r.Egress.Repair,
				Audit:  r.Egress.Audit,
				Usage:  r.Egress.Usage,
			},
			Ingress: Ingress{
				Repair: r.Ingress.Repaid,
				Usage:  r.Ingress.Usage,
			},
			Delete:        r.Delete,
			IntervalStart: r.IntervalStart,
		})
	}

	return Monthly{
		BandwidthDaily:   rollups,
		BandwidthSummary: bandwidth.Summary,
		EgressSummary:    egress.Summary,
		IngressSummary:   ingress.Summary,
	}, nil
}

// getMonthly returns monthly bandwidth summary for single node.
func (service *Service) getMonthly(ctx context.Context, node nodes.Node) (_ Monthly, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Monthly{}, nodes.ErrNodeNotReachable.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	bandwidthClient := multinodepb.NewDRPCBandwidthClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret[:],
	}

	ingress, err := bandwidthClient.IngressSummary(ctx, &multinodepb.IngressSummaryRequest{
		Header: header,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	egress, err := bandwidthClient.EgressSummary(ctx, &multinodepb.EgressSummaryRequest{
		Header: header,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	bandwidth, err := bandwidthClient.BandwidthSummary(ctx, &multinodepb.BandwidthSummaryRequest{
		Header: header,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	usageRollup, err := bandwidthClient.Daily(ctx, &multinodepb.DailyRequest{
		Header: header,
	})
	if err != nil {
		return Monthly{}, Error.Wrap(err)
	}

	var rollups []UsageRollup
	for _, r := range usageRollup.UsageRollup {
		rollups = append(rollups, UsageRollup{
			Egress: Egress{
				Repair: r.Egress.Repair,
				Audit:  r.Egress.Audit,
				Usage:  r.Egress.Usage,
			},
			Ingress: Ingress{
				Repair: r.Ingress.Repaid,
				Usage:  r.Ingress.Usage,
			},
			Delete:        r.Delete,
			IntervalStart: r.IntervalStart,
		})
	}

	return Monthly{
		BandwidthDaily:   rollups,
		BandwidthSummary: bandwidth.Summary,
		EgressSummary:    egress.Summary,
		IngressSummary:   ingress.Summary,
	}, nil
}
