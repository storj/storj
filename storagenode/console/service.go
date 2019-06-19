// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/storj/internal/version"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
)

type DB interface {
	GetSatelliteIDs(ctx context.Context) (storj.NodeIDList, error)
}

// Service is handling storage node operator related logic
type Service struct {
	log *zap.Logger

	bandwidth bandwidth.DB
	pieceInfo pieces.DB
	kademlia  *kademlia.Kademlia
	version   *version.Service

	allocatedBandwidth memory.Size
	allocatedDiskSpace memory.Size
	walletNumber       string
	startedAt          time.Time
	versionInfo        version.Info
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, bandwidth bandwidth.DB, pieceInfo pieces.DB, kademlia *kademlia.Kademlia, version *version.Service,
	allocatedBandwidth, allocatedDiskSpace memory.Size, walletNumber string, versionInfo version.Info) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if bandwidth == nil {
		return nil, errs.New("bandwidth can't be nil")
	}

	if pieceInfo == nil {
		return nil, errs.New("pieceInfo can't be nil")
	}

	if kademlia == nil {
		return nil, errs.New("kademlia can't be nil")
	}

	service := Service{
		log:                log,
		bandwidth:          bandwidth,
		pieceInfo:          pieceInfo,
		kademlia:           kademlia,
		allocatedBandwidth: allocatedBandwidth,
		allocatedDiskSpace: allocatedDiskSpace,
		walletNumber:       walletNumber,
		startedAt:          time.Now(),
		versionInfo:        versionInfo,
	}

	return &service, nil
}

// GetUsedBandwidth returns all info about storage node bandwidth usage
func (s *Service) GetUsedBandwidthTotal(ctx context.Context) (*BandwidthInfo, error) {
	usage, err := bandwidth.TotalMonthlySummary(ctx, s.bandwidth)
	if err != nil {
		return nil, err
	}

	return FromUsage(usage, s.allocatedBandwidth.Int64())
}

// GetBandwidthBySatellite returns all info about storage node bandwidth usage by satellite
func (s *Service) GetBandwidthBySatellite(ctx context.Context, satelliteID storj.NodeID) (_ *BandwidthInfo, err error) {
	summaries, err := s.bandwidth.SummaryBySatellite(ctx, time.Time{}, time.Now())
	if err != nil {
		return nil, err
	}

	// TODO: update bandwidth.DB with GetBySatellite
	return FromUsage(summaries[satelliteID], s.allocatedBandwidth.Int64())
}

// GetUsedStorageTotal returns all info about storagenode disk space usage
func (s *Service) GetUsedStorageTotal(ctx context.Context) (*DiskSpaceInfo, error) {
	spaceUsed, err := s.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.allocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}

// GetUsedStorageTotal returns all info about storagenode disk space usage
func (s *Service) GetUsedStorageBySatellite(ctx context.Context, satelliteID storj.NodeID) (*DiskSpaceInfo, error) {
	spaceUsed, err := s.pieceInfo.SpaceUsedBySatellite(ctx, satelliteID)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.allocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}

// GetWalletNumber return wallet number of node operator
func (s *Service) GetWalletNumber(ctx context.Context) string {
	return s.walletNumber
}

// GetUptime return wallet number of node operator
func (s *Service) GetUptime(ctx context.Context) time.Duration {
	return time.Now().Sub(s.startedAt)
}

// GetNodeID return current node id
func (s *Service) GetNodeID(ctx context.Context) storj.NodeID {
	return s.kademlia.Local().Id
}

// GetVersion return current node version
func (s *Service) GetVersion(ctx context.Context) version.Info {
	return s.versionInfo
}

// CheckVersion checks to make sure the version is still okay, returning an error if not
func (s *Service) CheckVersion(ctx context.Context) error {
	return s.version.CheckVersion(ctx)
}

func (s *Service) GetSatellites(ctx context.Context) (_ storj.NodeIDList, err error) {
	summaries, err := s.bandwidth.SummaryBySatellite(ctx, time.Time{}, time.Now())
	if err != nil {
		return nil, err
	}

	var satellites storj.NodeIDList
	for id := range summaries {
		satellites = append(satellites, id)
	}

	return satellites, nil
}

// getMonthRange is used to get first and last dates of month
func getMonthRange() (firstDay, lastDay time.Time) {
	now := time.Now()
	currentYear, currentMonth, _ := now.Date()
	currentLocation := now.Location()

	firstDay = time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastDay = firstDay.AddDate(0, 1, -1)

	return
}
