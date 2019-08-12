// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

// Stats encapsulates storagenode stats retrieved from the satellite
type Stats struct {
	SatelliteID storj.NodeID

	UptimeCheck ReputationStats
	AuditCheck  ReputationStats
}

// ReputationStats encapsulates storagenode reputation metrics
type ReputationStats struct {
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

// Client encapsulates NodeStatsClient with underlying connection
type Client struct {
	conn *grpc.ClientConn
	pb.NodeStatsClient
}

// Close closes underlying client connection
func (c *Client) Close() error {
	return c.conn.Close()
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

// GetStatsFromSatellite retrieves node stats from particular satellite
func (s *Service) GetStatsFromSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.DialNodeStats(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	defer func() {
		if cerr := client.Close(); cerr != nil {
			err = errs.Combine(err, NodeStatsServiceErr.New("failed to close connection: %v", cerr))
		}
	}()

	resp, err := client.GetStats(ctx, &pb.GetStatsRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	uptime := resp.GetUptimeCheck()
	audit := resp.GetAuditCheck()

	return &Stats{
		SatelliteID: satelliteID,
		UptimeCheck: ReputationStats{
			TotalCount:      uptime.GetTotalCount(),
			SuccessCount:    uptime.GetSuccessCount(),
			ReputationAlpha: uptime.GetReputationAlpha(),
			ReputationBeta:  uptime.GetReputationBeta(),
			ReputationScore: uptime.GetReputationScore(),
		},
		AuditCheck: ReputationStats{
			TotalCount:      audit.GetTotalCount(),
			SuccessCount:    audit.GetSuccessCount(),
			ReputationAlpha: audit.GetReputationAlpha(),
			ReputationBeta:  audit.GetReputationBeta(),
			ReputationScore: audit.GetReputationScore(),
		},
	}, nil
}

// GetDailyStorageUsedForSatellite returns daily SpaceUsageStamps over a period of time for a particular satellite
func (s *Service) GetDailyStorageUsedForSatellite(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []SpaceUsageStamp, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.DialNodeStats(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	defer func() {
		if cerr := client.Close(); cerr != nil {
			err = errs.Combine(err, NodeStatsServiceErr.New("failed to close connection: %v", cerr))
		}
	}()

	resp, err := client.DailyStorageUsage(ctx, &pb.DailyStorageUsageRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return fromSpaceUsageResponse(resp, satelliteID), nil
}

// DialNodeStats dials GRPC NodeStats client for the satellite by id
func (s *Service) DialNodeStats(ctx context.Context, satelliteID storj.NodeID) (*Client, error) {
	satellite, err := s.kademlia.FindNode(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %v", satelliteID, err)
	}

	conn, err := s.transport.DialNode(ctx, &satellite)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %v", satelliteID, err)
	}

	return &Client{
		conn:            conn,
		NodeStatsClient: pb.NewNodeStatsClient(conn),
	}, nil
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
