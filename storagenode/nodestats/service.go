// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dateutil"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode/console"
)

var (
	// NodeStatsServiceErr defines node stats service error
	NodeStatsServiceErr = errs.Class("node stats service error")

	mon = monkit.Package()
)

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

	statsLoop *sync2.Cycle
	spaceLoop *sync2.Cycle

	transport transport.Client
	consoleDB console.DB
	kademlia  *kademlia.Kademlia
}

// NewService creates new instance of service
func NewService(log *zap.Logger, transport transport.Client, consoleDB console.DB, kademlia *kademlia.Kademlia) *Service {
	return &Service{
		log:       log,
		statsLoop: sync2.NewCycle(time.Hour * 4),
		spaceLoop: sync2.NewCycle(time.Hour * 12),
		transport: transport,
		consoleDB: consoleDB,
		kademlia:  kademlia,
	}
}

// Run runs loop
func (s *Service) Run(ctx context.Context) error {
	var group errgroup.Group

	s.statsLoop.Start(ctx, &group, func(ctx context.Context) error {
		err := s.CacheStatsFromSatellites(ctx)
		if err != nil {
			s.log.Error("Get stats query failed", zap.Error(err))
		}

		return nil
	})
	s.spaceLoop.Start(ctx, &group, func(ctx context.Context) error {
		err := s.CacheSpaceUsageFromSatellites(ctx)
		if err != nil {
			s.log.Error("Get disk space usage query failed", zap.Error(err))
		}

		return nil
	})

	return group.Wait()
}

// CacheStatsFromSatellites queries node stats from all the satellites
// known to the storagenode and stores this information into db
func (s *Service) CacheStatsFromSatellites(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	satellites, err := s.consoleDB.Satellites().GetIDs(ctx)
	if err != nil {
		return NodeStatsServiceErr.Wrap(err)
	}

	var cacheStatsErr errs.Group
	for _, satellite := range satellites {
		stats, err := s.GetStatsFromSatellite(ctx, satellite)
		if err != nil {
			cacheStatsErr.Add(err)
			continue
		}

		stats.UpdatedAt = time.Now()

		// try to update stats from satellite
		if err = s.consoleDB.Stats().Update(ctx, *stats); err != nil {
			// if stats doesn't exists - create new one
			if err == sql.ErrNoRows {
				_, err = s.consoleDB.Stats().Create(ctx, *stats)
				if err != nil {
					cacheStatsErr.Add(NodeStatsServiceErr.Wrap(err))
					continue
				}
			}

			cacheStatsErr.Add(NodeStatsServiceErr.Wrap(err))
			continue
		}
	}

	return cacheStatsErr.Err()
}

// CacheSpaceUsageFromSatellites queries disk space usage from all the satellites
// known to the storagenode and stores this information into db
func (s *Service) CacheSpaceUsageFromSatellites(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	satellites, err := s.consoleDB.Satellites().GetIDs(ctx)
	if err != nil {
		return NodeStatsServiceErr.Wrap(err)
	}

	// get current month edges
	startDate, endDate := dateutil.MonthBoundary(time.Now().UTC())

	var cacheSpaceErr errs.Group
	for _, satellite := range satellites {
		spaceUsages, err := s.GetDailyStorageUsedForSatellite(ctx, satellite, startDate, endDate)
		if err != nil {
			cacheSpaceErr.Add(err)
			continue
		}

		err = s.consoleDB.DiskSpaceUsages().Store(ctx, spaceUsages)
		if err != nil {
			cacheSpaceErr.Add(NodeStatsServiceErr.Wrap(err))
			continue
		}
	}

	return cacheSpaceErr.Err()
}

// GetStatsFromSatellite retrieves node stats from particular satellite
func (s *Service) GetStatsFromSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *console.NodeStats, err error) {
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

	return &console.NodeStats{
		SatelliteID: satelliteID,
		UptimeCheck: console.ReputationStats{
			TotalCount:      uptime.GetTotalCount(),
			SuccessCount:    uptime.GetSuccessCount(),
			ReputationAlpha: uptime.GetReputationAlpha(),
			ReputationBeta:  uptime.GetReputationBeta(),
			ReputationScore: uptime.GetReputationScore(),
		},
		AuditCheck: console.ReputationStats{
			TotalCount:      audit.GetTotalCount(),
			SuccessCount:    audit.GetSuccessCount(),
			ReputationAlpha: audit.GetReputationAlpha(),
			ReputationBeta:  audit.GetReputationBeta(),
			ReputationScore: audit.GetReputationScore(),
		},
	}, nil
}

// GetDailyStorageUsedForSatellite returns daily SpaceUsageStamps over a period of time for a particular satellite
func (s *Service) GetDailyStorageUsedForSatellite(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []console.DiskSpaceUsage, err error) {
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

	resp, err := client.DailyStorageUsage(ctx, &pb.DailyStorageUsageRequest{From: from, To: to})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return fromSpaceUsageResponse(resp, satelliteID), nil
}

// DialNodeStats dials GRPC NodeStats client for the satellite by id
func (s *Service) DialNodeStats(ctx context.Context, satelliteID storj.NodeID) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

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

// fromSpaceUsageResponse get DiskSpaceUsage slice from pb.SpaceUsageResponse
func fromSpaceUsageResponse(resp *pb.DailyStorageUsageResponse, satelliteID storj.NodeID) []console.DiskSpaceUsage {
	var stamps []console.DiskSpaceUsage

	for _, pbUsage := range resp.GetDailyStorageUsage() {
		stamps = append(stamps, console.DiskSpaceUsage{
			SatelliteID: satelliteID,
			AtRestTotal: pbUsage.AtRestTotal,
			Timestamp:   pbUsage.Timestamp,
		})
	}

	return stamps
}

// Close closes underlying cycles
func (s *Service) Close() error {
	defer mon.Task()(nil)(nil)
	s.statsLoop.Close()
	s.spaceLoop.Close()
	return nil
}
