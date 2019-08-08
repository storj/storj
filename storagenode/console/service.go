// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	// SNOServiceErr defines sno service error
	SNOServiceErr = errs.Class("storage node dashboard service error")

	mon = monkit.Package()
)

// Storage encapsulates DBs used by console service
type Storage struct {
	Console      DB
	Bandwidth    bandwidth.DB
	Reputation   reputation.DB
	StorageUsage storageusage.DB
	PieceStore   *pieces.Store
	Kademlia     *kademlia.Kademlia
	Trust        *trust.Pool
}

// NodeInfo gathers static info about current node
type NodeInfo struct {
	Wallet             string
	Version            version.Info
	AllocatedBandwidth memory.Size
	AllocatedDiskSpace memory.Size
	StartedAt          time.Time
}

// DB exposes methods for managing SNO dashboard related data.
type DB interface {
	// Bandwidth is a getter for Bandwidth db
	Bandwidth() Bandwidth
}

// Service is handling storage node operator related logic
type Service struct {
	log *zap.Logger

	storage Storage
	version *version.Service

	info NodeInfo
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, storage Storage, version *version.Service, info NodeInfo) *Service {
	return &Service{
		log:     log,
		storage: storage,
		version: version,
		info:    info,
	}
}

// GetUsedBandwidthTotal returns all info about storage node bandwidth usage
func (s *Service) GetUsedBandwidthTotal(ctx context.Context) (_ *BandwidthInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	usage, err := bandwidth.TotalMonthlySummary(ctx, s.storage.Bandwidth)
	if err != nil {
		return nil, err
	}

	return FromUsage(usage, s.info.AllocatedBandwidth.Int64())
}

// GetDailyTotalBandwidthUsed returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order
func (s *Service) GetDailyTotalBandwidthUsed(ctx context.Context, from, to time.Time) (_ []BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.storage.Console.Bandwidth().GetDailyTotal(ctx, from, to)
}

// GetDailyBandwidthUsed returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order for particular satellite
func (s *Service) GetDailyBandwidthUsed(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.storage.Console.Bandwidth().GetDaily(ctx, satelliteID, from, to)
}

// GetBandwidthBySatellite returns all info about storage node bandwidth usage by satellite
func (s *Service) GetBandwidthBySatellite(ctx context.Context, satelliteID storj.NodeID) (_ *BandwidthInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	summaries, err := s.storage.Bandwidth.SummaryBySatellite(ctx, time.Time{}, time.Now())
	if err != nil {
		return nil, err
	}

	// TODO: update bandwidth.DB with GetBySatellite
	return FromUsage(summaries[satelliteID], s.info.AllocatedBandwidth.Int64())
}

// GetUsedStorageTotal returns all info about storagenode disk space usage
func (s *Service) GetUsedStorageTotal(ctx context.Context) (_ *DiskSpaceInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	spaceUsed, err := s.storage.PieceStore.SpaceUsedForPieces(ctx)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.info.AllocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}

// GetUsedStorageBySatellite returns all info about storagenode disk space usage
func (s *Service) GetUsedStorageBySatellite(ctx context.Context, satelliteID storj.NodeID) (_ *DiskSpaceInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	spaceUsed, err := s.storage.PieceStore.SpaceUsedBySatellite(ctx, satelliteID)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.info.AllocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}

// GetWalletAddress return wallet address of node operator
func (s *Service) GetWalletAddress(ctx context.Context) string {
	defer mon.Task()(&ctx)(nil)
	return s.info.Wallet
}

// GetUptime returns current storagenode uptime
func (s *Service) GetUptime(ctx context.Context) time.Duration {
	defer mon.Task()(&ctx)(nil)
	return time.Now().Sub(s.info.StartedAt)
}

// GetStatsFromSatellite returns storagenode stats from the satellite
func (s *Service) GetStatsFromSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *reputation.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.storage.Reputation.Get(ctx, satelliteID)
}

// GetDailyStorageUsedForSatellite returns daily SpaceUsageStamps for a particular satellite
func (s *Service) GetDailyStorageUsedForSatellite(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.storage.StorageUsage.GetDaily(ctx, satelliteID, from, to)
}

// GetNodeID return current node id
func (s *Service) GetNodeID(ctx context.Context) storj.NodeID {
	defer mon.Task()(&ctx)(nil)
	return s.storage.Kademlia.Local().Id
}

// GetVersion return current node version
func (s *Service) GetVersion(ctx context.Context) version.Info {
	defer mon.Task()(&ctx)(nil)
	return s.info.Version
}

// CheckVersion checks to make sure the version is still okay, returning an error if not
func (s *Service) CheckVersion(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return s.version.CheckVersion(ctx)
}

// GetSatellites used to retrieve satellites list
func (s *Service) GetSatellites(ctx context.Context) (_ storj.NodeIDList) {
	defer mon.Task()(&ctx)(nil)
	return s.storage.Trust.GetSatellites(ctx)
}
