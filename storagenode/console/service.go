// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"math"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/private/version"
	"storj.io/storj/private/date"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/heldamount"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	// SNOServiceErr defines sno service error.
	SNOServiceErr = errs.Class("storage node dashboard service error")

	mon = monkit.Package()
)

// Service is handling storage node operator related logic.
//
// architecture: Service
type Service struct {
	log            *zap.Logger
	trust          *trust.Pool
	bandwidthDB    bandwidth.DB
	reputationDB   reputation.DB
	storageUsageDB storageusage.DB
	pricingDB      pricing.DB
	satelliteDB    satellites.DB
	pieceStore     *pieces.Store
	contact        *contact.Service

	version   *checker.Service
	pingStats *contact.PingStats

	allocatedDiskSpace memory.Size

	walletAddress string
	startedAt     time.Time
	versionInfo   version.Info
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, bandwidth bandwidth.DB, pieceStore *pieces.Store, version *checker.Service,
	allocatedDiskSpace memory.Size, walletAddress string, versionInfo version.Info, trust *trust.Pool,
	reputationDB reputation.DB, storageUsageDB storageusage.DB, pricingDB pricing.DB, satelliteDB satellites.DB, pingStats *contact.PingStats, contact *contact.Service) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if bandwidth == nil {
		return nil, errs.New("bandwidth can't be nil")
	}

	if pieceStore == nil {
		return nil, errs.New("pieceStore can't be nil")
	}

	if version == nil {
		return nil, errs.New("version can't be nil")
	}

	if pingStats == nil {
		return nil, errs.New("pingStats can't be nil")
	}

	if contact == nil {
		return nil, errs.New("contact service can't be nil")
	}

	return &Service{
		log:                log,
		trust:              trust,
		bandwidthDB:        bandwidth,
		reputationDB:       reputationDB,
		storageUsageDB:     storageUsageDB,
		pricingDB:          pricingDB,
		satelliteDB:        satelliteDB,
		pieceStore:         pieceStore,
		version:            version,
		pingStats:          pingStats,
		allocatedDiskSpace: allocatedDiskSpace,
		contact:            contact,
		walletAddress:      walletAddress,
		startedAt:          time.Now(),
		versionInfo:        versionInfo,
	}, nil
}

// SatelliteInfo encapsulates satellite ID and disqualification.
type SatelliteInfo struct {
	ID           storj.NodeID `json:"id"`
	URL          string       `json:"url"`
	Disqualified *time.Time   `json:"disqualified"`
	Suspended    *time.Time   `json:"suspended"`
}

// Dashboard encapsulates dashboard stale data.
type Dashboard struct {
	NodeID storj.NodeID `json:"nodeID"`
	Wallet string       `json:"wallet"`

	Satellites []SatelliteInfo `json:"satellites"`

	DiskSpace DiskSpaceInfo `json:"diskSpace"`
	Bandwidth BandwidthInfo `json:"bandwidth"`

	LastPinged time.Time `json:"lastPinged"`

	Version        version.SemVer `json:"version"`
	AllowedVersion version.SemVer `json:"allowedVersion"`
	UpToDate       bool           `json:"upToDate"`

	StartedAt time.Time `json:"startedAt"`
}

// GetDashboardData returns stale dashboard data.
func (s *Service) GetDashboardData(ctx context.Context) (_ *Dashboard, err error) {
	defer mon.Task()(&ctx)(&err)
	data := new(Dashboard)

	data.NodeID = s.contact.Local().ID
	data.Wallet = s.walletAddress
	data.Version = s.versionInfo.Version
	data.StartedAt = s.startedAt

	data.LastPinged = s.pingStats.WhenLastPinged()
	data.AllowedVersion, data.UpToDate = s.version.IsAllowed(ctx)

	stats, err := s.reputationDB.All(ctx)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	for _, rep := range stats {
		url, err := s.trust.GetNodeURL(ctx, rep.SatelliteID)
		if err != nil {
			return nil, SNOServiceErr.Wrap(err)
		}

		data.Satellites = append(data.Satellites,
			SatelliteInfo{
				ID:           rep.SatelliteID,
				Disqualified: rep.Disqualified,
				Suspended:    rep.Suspended,
				URL:          url.Address,
			},
		)
	}

	pieceTotal, _, err := s.pieceStore.SpaceUsedForPieces(ctx)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	trash, err := s.pieceStore.SpaceUsedForTrash(ctx)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	bandwidthUsage, err := s.bandwidthDB.MonthSummary(ctx, time.Now())
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	data.DiskSpace = DiskSpaceInfo{
		Used:      pieceTotal,
		Available: s.allocatedDiskSpace.Int64(),
		Trash:     trash,
	}

	data.Bandwidth = BandwidthInfo{
		Used: bandwidthUsage,
	}

	return data, nil
}

// PriceModel is a satellite prices for storagenode usage TB/H.
type PriceModel struct {
	EgressBandwidth int64
	RepairBandwidth int64
	AuditBandwidth  int64
	DiskSpace       int64
}

// Satellite encapsulates satellite related data.
type Satellite struct {
	ID               storj.NodeID            `json:"id"`
	StorageDaily     []storageusage.Stamp    `json:"storageDaily"`
	BandwidthDaily   []bandwidth.UsageRollup `json:"bandwidthDaily"`
	StorageSummary   float64                 `json:"storageSummary"`
	BandwidthSummary int64                   `json:"bandwidthSummary"`
	EgressSummary    int64                   `json:"egressSummary"`
	IngressSummary   int64                   `json:"ingressSummary"`
	Audit            reputation.Metric       `json:"audit"`
	Uptime           reputation.Metric       `json:"uptime"`
	PriceModel       PriceModel              `json:"priceModel"`
	NodeJoinedAt     time.Time               `json:"nodeJoinedAt"`
}

// GetSatelliteData returns satellite related data.
func (s *Service) GetSatelliteData(ctx context.Context, satelliteID storj.NodeID) (_ *Satellite, err error) {
	defer mon.Task()(&ctx)(&err)
	from, to := date.MonthBoundary(time.Now().UTC())

	bandwidthDaily, err := s.bandwidthDB.GetDailySatelliteRollups(ctx, satelliteID, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	storageDaily, err := s.storageUsageDB.GetDaily(ctx, satelliteID, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	bandwidthSummary, err := s.bandwidthDB.SatelliteSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	egressSummary, err := s.bandwidthDB.SatelliteEgressSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	ingressSummary, err := s.bandwidthDB.SatelliteIngressSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	storageSummary, err := s.storageUsageDB.SatelliteSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	rep, err := s.reputationDB.Get(ctx, satelliteID)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	pricingModel, err := s.pricingDB.Get(ctx, satelliteID)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	satellitePricing := PriceModel{
		EgressBandwidth: pricingModel.EgressBandwidth,
		RepairBandwidth: pricingModel.RepairBandwidth,
		AuditBandwidth:  pricingModel.AuditBandwidth,
		DiskSpace:       pricingModel.DiskSpace,
	}

	return &Satellite{
		ID:               satelliteID,
		StorageDaily:     storageDaily,
		BandwidthDaily:   bandwidthDaily,
		StorageSummary:   storageSummary,
		BandwidthSummary: bandwidthSummary.Total(),
		EgressSummary:    egressSummary.Total(),
		IngressSummary:   ingressSummary.Total(),
		Audit:            rep.Audit,
		Uptime:           rep.Uptime,
		PriceModel:       satellitePricing,
		NodeJoinedAt:     rep.JoinedAt,
	}, nil
}

// Satellites represents consolidated data across all satellites.
type Satellites struct {
	StorageDaily     []storageusage.Stamp    `json:"storageDaily"`
	BandwidthDaily   []bandwidth.UsageRollup `json:"bandwidthDaily"`
	StorageSummary   float64                 `json:"storageSummary"`
	BandwidthSummary int64                   `json:"bandwidthSummary"`
	EgressSummary    int64                   `json:"egressSummary"`
	IngressSummary   int64                   `json:"ingressSummary"`
	EarliestJoinedAt time.Time               `json:"earliestJoinedAt"`
}

// GetAllSatellitesData returns bandwidth and storage daily usage consolidate
// among all satellites from the node's trust pool.
func (s *Service) GetAllSatellitesData(ctx context.Context) (_ *Satellites, err error) {
	defer mon.Task()(&ctx)(nil)
	from, to := date.MonthBoundary(time.Now().UTC())

	bandwidthDaily, err := s.bandwidthDB.GetDailyRollups(ctx, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	storageDaily, err := s.storageUsageDB.GetDailyTotal(ctx, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	bandwidthSummary, err := s.bandwidthDB.Summary(ctx, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	egressSummary, err := s.bandwidthDB.EgressSummary(ctx, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	ingressSummary, err := s.bandwidthDB.IngressSummary(ctx, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	storageSummary, err := s.storageUsageDB.Summary(ctx, from, to)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	satellitesIDs := s.trust.GetSatellites(ctx)
	joinedAt := time.Now().UTC()
	for i := 0; i < len(satellitesIDs); i++ {
		stats, err := s.reputationDB.Get(ctx, satellitesIDs[i])
		if err != nil {
			return nil, SNOServiceErr.Wrap(err)
		}

		if !stats.JoinedAt.IsZero() && stats.JoinedAt.Before(joinedAt) {
			joinedAt = stats.JoinedAt
		}
	}

	return &Satellites{
		StorageDaily:     storageDaily,
		BandwidthDaily:   bandwidthDaily,
		StorageSummary:   storageSummary,
		BandwidthSummary: bandwidthSummary.Total(),
		EgressSummary:    egressSummary.Total(),
		IngressSummary:   ingressSummary.Total(),
		EarliestJoinedAt: joinedAt,
	}, nil
}

// VerifySatelliteID verifies if the satellite belongs to the trust pool.
func (s *Service) VerifySatelliteID(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.trust.VerifySatelliteID(ctx, satelliteID)
	if err != nil {
		return SNOServiceErr.Wrap(err)
	}

	return nil
}

// GetSatelliteEstimatedPayout returns estimated payout for current and previous months from specific satellite with current level of load.
func (s *Service) GetSatelliteEstimatedPayout(ctx context.Context, satelliteID storj.NodeID) (payout heldamount.EstimatedPayout, err error) {
	defer mon.Task()(&ctx)(&err)

	currentMonthPayout, err := s.estimatedPayoutMonthly(ctx, satelliteID, time.Now().UTC())
	if err != nil {
		return heldamount.EstimatedPayout{}, SNOServiceErr.Wrap(err)
	}

	previousMonthPayout, err := s.estimatedPayoutMonthly(ctx, satelliteID, time.Now().UTC().AddDate(0, -1, 0).UTC())
	if err != nil {
		return heldamount.EstimatedPayout{}, SNOServiceErr.Wrap(err)
	}

	payout.CurrentMonth = currentMonthPayout
	payout.PreviousMonth = previousMonthPayout

	return payout, nil
}

// GetAllSatellitesEstimatedPayout returns estimated payout for current and previous months from all satellites with current level of load.
func (s *Service) GetAllSatellitesEstimatedPayout(ctx context.Context) (payout heldamount.EstimatedPayout, err error) {
	defer mon.Task()(&ctx)(&err)

	satelliteIDs := s.trust.GetSatellites(ctx)
	for i := 0; i < len(satelliteIDs); i++ {
		current, err := s.estimatedPayoutMonthly(ctx, satelliteIDs[i], time.Now().UTC())
		if err != nil {
			return heldamount.EstimatedPayout{}, SNOServiceErr.Wrap(err)
		}

		previous, err := s.estimatedPayoutMonthly(ctx, satelliteIDs[i], time.Now().UTC().AddDate(0, -1, 0).UTC())
		if err != nil {
			return heldamount.EstimatedPayout{}, SNOServiceErr.Wrap(err)
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

// estimatedPayoutMonthly returns estimated payout data monthly from specific satellite.
func (s *Service) estimatedPayoutMonthly(ctx context.Context, satelliteID storj.NodeID, month time.Time) (payoutData heldamount.PayoutMonthly, err error) {
	defer mon.Task()(&ctx)(&err)

	priceModel, err := s.pricingDB.Get(ctx, satelliteID)
	if err != nil {
		return heldamount.PayoutMonthly{}, SNOServiceErr.Wrap(err)
	}

	stats, err := s.reputationDB.Get(ctx, satelliteID)
	if err != nil {
		return heldamount.PayoutMonthly{}, SNOServiceErr.Wrap(err)
	}

	heldRate := s.getHeldRate(stats.JoinedAt)
	payoutData.HeldRate = heldRate

	from, to := date.MonthBoundary(month)

	bandwidthDaily, err := s.bandwidthDB.GetDailySatelliteRollups(ctx, satelliteID, from, to)
	if err != nil {
		return heldamount.PayoutMonthly{}, SNOServiceErr.Wrap(err)
	}

	for i := 0; i < len(bandwidthDaily); i++ {
		payoutData.EgressBandwidth += bandwidthDaily[i].Egress.Usage
		payoutData.EgressRepairAudit += bandwidthDaily[i].Egress.Audit + bandwidthDaily[i].Egress.Repair
	}
	payoutData.EgressBandwidthPayout += int64(float64(payoutData.EgressBandwidth*priceModel.EgressBandwidth) / math.Pow10(12))
	payoutData.EgressRepairAuditPayout += int64(float64(payoutData.EgressRepairAudit*priceModel.AuditBandwidth) / math.Pow10(12))

	storageDaily, err := s.storageUsageDB.GetDaily(ctx, satelliteID, from, to)
	if err != nil {
		return heldamount.PayoutMonthly{}, SNOServiceErr.Wrap(err)
	}

	for j := 0; j < len(storageDaily); j++ {
		// dividing by 720 to show tbm instead of tbh
		payoutData.DiskSpace += storageDaily[j].AtRestTotal / 720
	}
	payoutData.DiskSpacePayout += int64(payoutData.DiskSpace / 720 / math.Pow10(12) * float64(priceModel.DiskSpace))

	payoutData.Held = (payoutData.DiskSpacePayout + payoutData.EgressBandwidthPayout + payoutData.EgressRepairAuditPayout) * heldRate / 100
	payoutData.Payout = payoutData.DiskSpacePayout + payoutData.EgressBandwidthPayout + payoutData.EgressRepairAuditPayout - payoutData.Held

	return payoutData, nil
}

func (s *Service) getHeldRate(joinTime time.Time) (heldRate int64) {
	monthsSinceJoin := date.MonthsCountSince(joinTime)
	switch monthsSinceJoin {
	case 0, 1, 2:
		heldRate = 75
	case 3, 4, 5:
		heldRate = 50
	case 6, 7, 8:
		heldRate = 25
	default:
		heldRate = 0
	}

	return heldRate
}
