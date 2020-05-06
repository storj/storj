// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
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

	LastPinged          time.Time    `json:"lastPinged"`
	LastPingFromID      storj.NodeID `json:"lastPingFromID"`
	LastPingFromAddress string       `json:"lastPingFromAddress"`

	Version        version.SemVer `json:"version"`
	AllowedVersion version.SemVer `json:"allowedVersion"`
	UpToDate       bool           `json:"upToDate"`

	StartedAt time.Time `json:"startedAt"`
}

// GetDashboardData returns stale dashboard data.
func (s *Service) GetDashboardData(ctx context.Context) (_ *Dashboard, err error) {
	defer mon.Task()(&ctx)(&err)
	data := new(Dashboard)

	data.NodeID = s.contact.Local().Id
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
		url, err := s.trust.GetAddress(ctx, rep.SatelliteID)
		if err != nil {
			return nil, SNOServiceErr.Wrap(err)
		}

		data.Satellites = append(data.Satellites,
			SatelliteInfo{
				ID:           rep.SatelliteID,
				Disqualified: rep.Disqualified,
				Suspended:    rep.Suspended,
				URL:          url,
			},
		)
	}

	_, piecesContentSize, err := s.pieceStore.SpaceUsedForPieces(ctx)
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
		Used:      piecesContentSize,
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
