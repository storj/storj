// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operator

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/monitor"
)

// Service is handling storage node operator related logic
type Service struct {
	log *zap.Logger

	bandwidth bandwidth.DB
	monitor   *monitor.Service
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, bandwidth bandwidth.DB, monitor *monitor.Service) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if bandwidth == nil {
		return nil, errs.New("bandwidth can't be nil")
	}

	if monitor == nil {
		return nil, errs.New("monitor can't be nil")
	}

	return &Service{log: log, bandwidth: bandwidth, monitor: monitor}, nil
}

// GetBandwidth returns all info about storage node bandwidth usage
func (s *Service) GetBandwidth(ctx context.Context, from, to time.Time) (*BandwidthInfo, error) {
	usage, err := s.bandwidth.Summary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	avaiableBandwidth, err := s.monitor.AvailableBandwidth(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: finish implementation
	return FromUsage(usage, avaiableBandwidth), nil
}
