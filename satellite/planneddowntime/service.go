// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package planneddowntime

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

type Service struct {
	log *zap.Logger
	db  overlay.DB
}

func NewService(log *zap.Logger, db overlay.DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// ScheduleDowntime inserts a downtime into the DB.
func (service *Service) ScheduleDowntime(ctx context.Context, id storj.NodeID, req *pb.ScheduleDowntimeRequest) (_ *pb.ScheduleDowntimeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = service.db.AddPlannedDowntime(ctx, id, req.Timeframe.Start, req.Timeframe.End)
	if err != nil {
		return nil, err
	}
	return &pb.ScheduleDowntimeResponse{
		Window: &pb.DowntimeWindow{
			Timeframe: &pb.Timeframe{
				Start: req.Timeframe.Start,
				End:   req.Timeframe.End,
			},
		},
	}, err
}

// Cancel deletes a scheduled timeframe from the DB.
func (service *Service) Cancel(ctx context.Context, id storj.NodeID, req *pb.CancelRequest) (_ *pb.CancelResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = service.db.CancelPlannedDowntime(ctx, id)
	if err != nil {
		return nil, err
	}
	return &pb.CancelResponse{}, nil
}

// Close closes resources.
func (service *Service) Close() error { return nil }
