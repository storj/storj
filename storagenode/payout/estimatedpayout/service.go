// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayout

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode/bandwidth"
	payout2 "storj.io/storj/storagenode/payout"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	// EstimationServiceErr defines sno service error.
	EstimationServiceErr = errs.Class("storage node estimation payout service error")

	mon = monkit.Package()
)

// Service is handling storage node estimation payout logic.
//
// architecture: Service
type Service struct {
	bandwidthDB    bandwidth.DB
	reputationDB   reputation.DB
	storageUsageDB storageusage.DB
	pricingDB      pricing.DB
	satelliteDB    satellites.DB
	trust          *trust.Pool
}

// NewService returns new instance of Service.
func NewService(bandwidthDB bandwidth.DB, reputationDB reputation.DB, storageUsageDB storageusage.DB, pricingDB pricing.DB, satelliteDB satellites.DB, trust *trust.Pool) *Service {
	return &Service{
		bandwidthDB:    bandwidthDB,
		reputationDB:   reputationDB,
		storageUsageDB: storageUsageDB,
		pricingDB:      pricingDB,
		satelliteDB:    satelliteDB,
		trust:          trust,
	}
}

// GetSatelliteEstimatedPayout returns estimated payout for current and previous months from specific satellite with current level of load.
func (s *Service) GetSatelliteEstimatedPayout(ctx context.Context, satelliteID storj.NodeID) (payout EstimatedPayout, err error) {
	defer mon.Task()(&ctx)(&err)

	currentMonthPayout, previousMonthPayout, err := s.estimatedPayout(ctx, satelliteID)
	if err != nil {
		return EstimatedPayout{}, EstimationServiceErr.Wrap(err)
	}

	payout.CurrentMonth = currentMonthPayout
	payout.PreviousMonth = previousMonthPayout

	return payout, nil
}

// GetAllSatellitesEstimatedPayout returns estimated payout for current and previous months from all satellites with current level of load.
func (s *Service) GetAllSatellitesEstimatedPayout(ctx context.Context) (payout EstimatedPayout, err error) {
	defer mon.Task()(&ctx)(&err)

	satelliteIDs := s.trust.GetSatellites(ctx)
	for i := 0; i < len(satelliteIDs); i++ {
		current, previous, err := s.estimatedPayout(ctx, satelliteIDs[i])
		if err != nil {
			return EstimatedPayout{}, EstimationServiceErr.Wrap(err)
		}

		payout.CurrentMonth.Payout += current.Payout
		payout.CurrentMonth.EgressRepairAuditPayout += current.EgressRepairAuditPayout
		payout.CurrentMonth.DiskSpacePayout += current.DiskSpacePayout
		payout.CurrentMonth.DiskSpace += current.DiskSpace
		payout.CurrentMonth.EgressBandwidth += current.EgressBandwidth
		payout.CurrentMonth.EgressBandwidthPayout += current.EgressBandwidthPayout
		payout.CurrentMonth.EgressRepairAudit += current.EgressRepairAudit
		payout.CurrentMonth.Held += current.Held
		payout.PreviousMonth.Payout += previous.Payout
		payout.PreviousMonth.DiskSpacePayout += previous.DiskSpacePayout
		payout.PreviousMonth.DiskSpace += previous.DiskSpace
		payout.PreviousMonth.EgressBandwidth += previous.EgressBandwidth
		payout.PreviousMonth.EgressBandwidthPayout += previous.EgressBandwidthPayout
		payout.PreviousMonth.EgressRepairAuditPayout += previous.EgressRepairAuditPayout
		payout.PreviousMonth.EgressRepairAudit += previous.EgressRepairAudit
		payout.PreviousMonth.Held += previous.Held
	}

	return payout, nil
}

// estimatedPayout returns estimated payout data for current and previous months from specific satellite.
func (s *Service) estimatedPayout(ctx context.Context, satelliteID storj.NodeID) (currentMonthPayout PayoutMonthly, previousMonthPayout PayoutMonthly, err error) {
	defer mon.Task()(&ctx)(&err)

	priceModel, err := s.pricingDB.Get(ctx, satelliteID)
	if err != nil {
		return PayoutMonthly{}, PayoutMonthly{}, EstimationServiceErr.Wrap(err)
	}

	stats, err := s.reputationDB.Get(ctx, satelliteID)
	if err != nil {
		return PayoutMonthly{}, PayoutMonthly{}, EstimationServiceErr.Wrap(err)
	}

	currentMonthPayout, err = s.estimationUsagePeriod(ctx, time.Now().UTC(), stats.JoinedAt, priceModel)
	previousMonthPayout, err = s.estimationUsagePeriod(ctx, time.Now().UTC().AddDate(0, -1, 0), stats.JoinedAt, priceModel)

	return currentMonthPayout, previousMonthPayout, nil
}

// estimationUsagePeriod returns PayoutMonthly for current satellite and current or previous month.
func (s *Service) estimationUsagePeriod(ctx context.Context, period time.Time, joinedAt time.Time, priceModel *pricing.Pricing) (payout PayoutMonthly, err error) {
	var from, to time.Time

	heldRate := payout2.GetHeldRate(joinedAt, period)
	payout.HeldRate = heldRate

	from, to = date.MonthBoundary(period)

	bandwidthDaily, err := s.bandwidthDB.GetDailySatelliteRollups(ctx, priceModel.SatelliteID, from, to)
	if err != nil {
		return PayoutMonthly{}, EstimationServiceErr.Wrap(err)
	}

	for i := 0; i < len(bandwidthDaily); i++ {
		payout.EgressBandwidth += bandwidthDaily[i].Egress.Usage
		payout.EgressRepairAudit += bandwidthDaily[i].Egress.Audit + bandwidthDaily[i].Egress.Repair
	}
	payout.SetEgressBandwidthPayout(priceModel.EgressBandwidth)
	payout.SetEgressRepairAuditPayout(priceModel.AuditBandwidth)

	storageDaily, err := s.storageUsageDB.GetDaily(ctx, priceModel.SatelliteID, from, to)
	if err != nil {
		return PayoutMonthly{}, EstimationServiceErr.Wrap(err)
	}

	for j := 0; j < len(storageDaily); j++ {
		payout.DiskSpace += storageDaily[j].AtRestTotal
	}
	// dividing by 720 to show tbm instead of tbh.
	payout.DiskSpace /= 720
	payout.SetDiskSpacePayout(priceModel.DiskSpace)
	payout.SetHeldAmount()
	payout.SetPayout()

	return payout, EstimationServiceErr.Wrap(err)
}
