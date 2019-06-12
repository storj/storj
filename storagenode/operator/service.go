// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operator

import (
	"context"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
)

// Service is handling storage node operator related logic
type Service struct {
	log *zap.Logger

	bandwidth bandwidth.DB
	pieceInfo pieces.DB

	allocatedBandwidth memory.Size
	allocatedDiskSpace memory.Size
	walletNumber       string
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, bandwidth bandwidth.DB, pieceInfo pieces.DB, allocatedBandwidth, allocatedDiskSpace memory.Size, walletNumber string) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if bandwidth == nil {
		return nil, errs.New("bandwidth can't be nil")
	}

	if pieceInfo == nil {
		return nil, errs.New("pieceInfo can't be nil")
	}

	service := Service{
		log:                log,
		bandwidth:          bandwidth,
		pieceInfo:          pieceInfo,
		allocatedBandwidth: allocatedBandwidth,
		allocatedDiskSpace: allocatedDiskSpace,
		walletNumber:       walletNumber,
	}

	return &service, nil
}

// GetUsedBandwidth returns all info about storage node bandwidth usage
func (s *Service) GetUsedBandwidth(ctx context.Context) (*BandwidthInfo, error) {
	usage, err := bandwidth.TotalMonthlySummary(ctx, s.bandwidth)
	if err != nil {
		return nil, err
	}

	return FromUsage(usage, s.allocatedBandwidth.Int64())
}

// GetUsedStorage returns all info about storagenode disk space usage
func (s *Service) GetUsedStorage(ctx context.Context) (*DiskSpaceInfo, error) {
	spaceUsed, err := s.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{Available: s.allocatedDiskSpace.Int64() - spaceUsed, Used: spaceUsed}, nil
}
