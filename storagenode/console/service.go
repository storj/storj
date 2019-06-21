// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/pieces"
)

var (
	// SNOServiceErr defines sno service error
	SNOServiceErr = errs.Class("storage node dashboard service error")

	mon = monkit.Package()
)

// DB exposes methods for managing SNO dashboard related data.
type DB interface {
	GetSatelliteIDs(ctx context.Context, from, to time.Time) (storj.NodeIDList, error)
}

// Service is handling storage node operator related logic
type Service struct {
	log *zap.Logger

	consoleDB   DB
	bandwidthDB bandwidth.DB
	pieceInfoDB pieces.DB
	kademlia    *kademlia.Kademlia
	version     *version.Service
	nodestats   *nodestats.Service

	allocatedBandwidth memory.Size
	allocatedDiskSpace memory.Size
	walletAddress      string
	startedAt          time.Time
	versionInfo        version.Info
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, consoleDB DB, bandwidth bandwidth.DB, pieceInfo pieces.DB, kademlia *kademlia.Kademlia, version *version.Service,
	nodestats *nodestats.Service, allocatedBandwidth, allocatedDiskSpace memory.Size, walletAddress string, versionInfo version.Info) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if consoleDB == nil {
		return nil, errs.New("consoleDB can't be nil")
	}

	if bandwidth == nil {
		return nil, errs.New("bandwidth can't be nil")
	}

	if pieceInfo == nil {
		return nil, errs.New("pieceInfo can't be nil")
	}

	if version == nil {
		return nil, errs.New("version can't be nil")
	}

	if kademlia == nil {
		return nil, errs.New("kademlia can't be nil")
	}

	return &Service{
		log:                log,
		consoleDB:          consoleDB,
		bandwidthDB:        bandwidth,
		pieceInfoDB:        pieceInfo,
		kademlia:           kademlia,
		version:            version,
		nodestats:          nodestats,
		allocatedBandwidth: allocatedBandwidth,
		allocatedDiskSpace: allocatedDiskSpace,
		walletAddress:      walletAddress,
		startedAt:          time.Now(),
		versionInfo:        versionInfo,
	}, nil
}

// GetUsedBandwidthTotal returns all info about storage node bandwidth usage
func (s *Service) GetUsedBandwidthTotal(ctx context.Context) (_ *BandwidthInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	usage, err := bandwidth.TotalMonthlySummary(ctx, s.bandwidthDB)
	if err != nil {
		return nil, err
	}

	return FromUsage(usage, s.allocatedBandwidth.Int64())
}

// GetBandwidthBySatellite returns all info about storage node bandwidth usage by satellite
func (s *Service) GetBandwidthBySatellite(ctx context.Context, satelliteID storj.NodeID) (_ *BandwidthInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	summaries, err := s.bandwidthDB.SummaryBySatellite(ctx, time.Time{}, time.Now())
	if err != nil {
		return nil, err
	}

	// TODO: update bandwidth.DB with GetBySatellite
	return FromUsage(summaries[satelliteID], s.allocatedBandwidth.Int64())
}

// GetUsedStorageTotal returns all info about storagenode disk space usage
func (s *Service) GetUsedStorageTotal(ctx context.Context) (_ *DiskSpaceInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	spaceUsed, err := s.pieceInfoDB.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.allocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}

// GetUsedStorageBySatellite returns all info about storagenode disk space usage
func (s *Service) GetUsedStorageBySatellite(ctx context.Context, satelliteID storj.NodeID) (_ *DiskSpaceInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	spaceUsed, err := s.pieceInfoDB.SpaceUsedBySatellite(ctx, satelliteID)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.allocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}

// GetWalletAddress return wallet address of node operator
func (s *Service) GetWalletAddress(ctx context.Context) string {
	defer mon.Task()(&ctx)(nil)
	return s.walletAddress
}

// GetUptime returns current storagenode uptime
func (s *Service) GetUptime(ctx context.Context) time.Duration {
	defer mon.Task()(&ctx)(nil)
	return time.Now().Sub(s.startedAt)
}

// GetUptimeCheckForSatellite returns uptime check for the satellite
func (s *Service) GetUptimeCheckForSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *nodestats.UptimeCheck, err error) {
	defer mon.Task()(&ctx)(&err)

	uptime, err := s.nodestats.GetUptimeCheckForSatellite(ctx, satelliteID)
	if err != nil {
		return nil, SNOServiceErr.Wrap(err)
	}

	return uptime, nil
}

// GetNodeID return current node id
func (s *Service) GetNodeID(ctx context.Context) storj.NodeID {
	defer mon.Task()(&ctx)(nil)
	return s.kademlia.Local().Id
}

// GetVersion return current node version
func (s *Service) GetVersion(ctx context.Context) version.Info {
	defer mon.Task()(&ctx)(nil)
	return s.versionInfo
}

// CheckVersion checks to make sure the version is still okay, returning an error if not
func (s *Service) CheckVersion(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return s.version.CheckVersion(ctx)
}

// GetSatellites used to retrieve satellites list
func (s *Service) GetSatellites(ctx context.Context) (_ storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.consoleDB.GetSatelliteIDs(ctx, time.Time{}, time.Now())
}
