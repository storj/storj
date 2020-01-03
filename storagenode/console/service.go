// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/reputation"
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
	pieceStore     *pieces.Store
	contact        *contact.Service

	version   *checker.Service
	pingStats *contact.PingStats

	allocatedBandwidth memory.Size
	allocatedDiskSpace memory.Size

	walletAddress string
	startedAt     time.Time
	versionInfo   version.Info
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, bandwidth bandwidth.DB, pieceStore *pieces.Store, version *checker.Service,
	allocatedBandwidth, allocatedDiskSpace memory.Size, walletAddress string, versionInfo version.Info, trust *trust.Pool,
	reputationDB reputation.DB, storageUsageDB storageusage.DB, pingStats *contact.PingStats, contact *contact.Service) (*Service, error) {
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
		pieceStore:         pieceStore,
		version:            version,
		pingStats:          pingStats,
		allocatedBandwidth: allocatedBandwidth,
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
				URL:          url,
			},
		)
	}

	spaceUsage, err := s.pieceStore.SpaceUsedForPieces(ctx)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	bandwidthUsage, err := s.bandwidthDB.MonthSummary(ctx)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	data.DiskSpace = DiskSpaceInfo{
		Used:      spaceUsage,
		Available: s.allocatedDiskSpace.Int64(),
	}

	data.Bandwidth = BandwidthInfo{
		Used:      bandwidthUsage,
		Available: s.allocatedBandwidth.Int64(),
	}

	return data, nil
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

	return &Satellites{
		StorageDaily:     storageDaily,
		BandwidthDaily:   bandwidthDaily,
		StorageSummary:   storageSummary,
		BandwidthSummary: bandwidthSummary.Total(),
		EgressSummary:    egressSummary.Total(),
		IngressSummary:   ingressSummary.Total(),
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
