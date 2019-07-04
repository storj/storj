// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	// NodeStatsServiceErr defines node stats service error
	NodeStatsServiceErr = errs.Class("node stats service error")

	mon = monkit.Package()
)

// UptimeCheck encapsulates storagenode uptime metrics
type UptimeCheck struct {
	TotalCount   int64
	SuccessCount int64

	ReputationAlpha float64
	ReputationBeta  float64
	ReputationScore float64
}

// SpaceUsageStamp is space usage for satellite at some point in time
type SpaceUsageStamp struct {
	SatelliteID storj.NodeID
	AtRestTotal float64

	TimeStamp time.Time
}

// Service retrieves info from satellites using GRPC client
type Service struct {
	log *zap.Logger

	transport transport.Client
	kademlia  *kademlia.Kademlia
}

// NewService creates new instance of service
func NewService(log *zap.Logger, transport transport.Client, kademlia *kademlia.Kademlia) *Service {
	return &Service{
		log:       log,
		transport: transport,
		kademlia:  kademlia,
	}
}

// GetUptimeCheckForSatellite retrieves UptimeChecks from particular satellite
func (s *Service) GetUptimeCheckForSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *UptimeCheck, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.getGRPCClientForSatellite(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	resp, err := client.UptimeCheck(ctx, &pb.UptimeCheckRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return &UptimeCheck{
		TotalCount:      resp.GetTotalCount(),
		SuccessCount:    resp.GetSuccessCount(),
		ReputationAlpha: resp.GetReputationAlpha(),
		ReputationBeta:  resp.GetReputationBeta(),
		ReputationScore: resp.GetReputationScore(),
	}, nil
}

// GetDailyStorageUsedForSatellite returns daily SpaceUsageStamps over a period of time for a particular satellite
func (s *Service) GetDailyStorageUsedForSatellite(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []SpaceUsageStamp, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.getGRPCClientForSatellite(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	resp, err := client.DailyStorageUsage(ctx, &pb.DailyStorageUsageRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return fromSpaceUsageResponse(resp, satelliteID), nil
}

// getGRPCClientForSatellite inits GRPC client for the satellite by id
func (s *Service) getGRPCClientForSatellite(ctx context.Context, satelliteID storj.NodeID) (pb.NodeStatsClient, error) {
	satellite, err := s.kademlia.FindNode(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %v", satelliteID, err)
	}

	conn, err := s.transport.DialNode(ctx, &satellite)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %v", satelliteID, err)
	}

	return pb.NewNodeStatsClient(conn), nil
}

// fromSpaceUsageResponse get SpaceUsageStamp slice from pb.SpaceUsageResponse
func fromSpaceUsageResponse(resp *pb.DailyStorageUsageResponse, satelliteID storj.NodeID) []SpaceUsageStamp {
	var stamps []SpaceUsageStamp

	for _, pbUsage := range resp.GetDailyStorageUsage() {
		stamps = append(stamps, SpaceUsageStamp{
			SatelliteID: satelliteID,
			AtRestTotal: pbUsage.AtRestTotal,
			TimeStamp:   pbUsage.TimeStamp,
		})
	}

	return stamps
}
