// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operator

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/pieces"
)

// Service is handling storage node operator related logic
type Service struct {
	log *zap.Logger

	bandwidth bandwidth.DB
	monitor   *monitor.Service
	pieceInfo pieces.DB
	//AllocatedBandwidth memory.Size
	AllocatedDiskSpace memory.Size
	walletNumber       string
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, bandwidth bandwidth.DB, monitor *monitor.Service, pieceInfo pieces.DB, walletNumber string) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if bandwidth == nil {
		return nil, errs.New("bandwidth can't be nil")
	}

	if monitor == nil {
		return nil, errs.New("monitor can't be nil")
	}

	if pieceInfo == nil {
		return nil, errs.New("pieceInfo can't be nil")
	}

	return &Service{log: log, bandwidth: bandwidth, monitor: monitor, pieceInfo: pieceInfo, walletNumber: walletNumber}, nil
}

// GetUsedBandwidth returns all info about storage node bandwidth usage
func (s *Service) GetUsedBandwidth(ctx context.Context) (*BandwidthInfo, error) {
	firstDayOfMonth, lastDayOfMonth := getMonthRange()

	usage, err := s.bandwidth.Summary(ctx, firstDayOfMonth, lastDayOfMonth)
	if err != nil {
		return nil, err
	}

	availableBandwidth, err := s.monitor.AvailableBandwidth(ctx)
	if err != nil {
		return nil, err
	}

	bandwidth, err := FromUsage(usage, availableBandwidth)
	if err != nil {
		return nil, err
	}

	return bandwidth, nil
}

// GetUsedStorage returns all info about storagenode disk space usage
func (s *Service) GetUsedStorage(ctx context.Context) (*DiskSpaceInfo, error) {
	spaceAvailable, err := s.monitor.AvailableSpace(ctx)
	if err != nil {
		return nil, err
	}

	spaceUsed, err := s.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: spaceAvailable, Used: spaceUsed}, nil
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
