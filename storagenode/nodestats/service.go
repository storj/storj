// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

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

	statsTicker *time.Ticker
	spaceTicker *time.Ticker

	transport transport.Client
	consoleDB console.DB
	kademlia  *kademlia.Kademlia
}

// NewService creates new instance of service
func NewService(log *zap.Logger, transport transport.Client, consoleDB console.DB, kademlia *kademlia.Kademlia) *Service {
	return &Service{
		log:         log,
		statsTicker: time.NewTicker(time.Second * 30),
		spaceTicker: time.NewTicker(time.Second * 60),
		transport:   transport,
		consoleDB:   consoleDB,
		kademlia:    kademlia,
	}
}

// Run runs loop
func (s *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		select {
		// handle cancellation signal first
		case <-ctx.Done():
			return ctx.Err()
		// wait for the next cache interval to happen
		case <-s.statsTicker.C:
			err = s.CacheStatsFromSatellites(ctx)
			if err != nil {
				s.log.Error(fmt.Sprintf("Get stats query failed: %v", err))
			}
		// wait for the next space interval to happen
		case <-s.spaceTicker.C:
			err = s.CacheSpaceUsageFromSatellites(ctx)
			if err != nil {
				s.log.Error(fmt.Sprintf("Get disk space usage query failed: %v", err))
			}
		}
	}
}

// CacheStatsFromSatellites queries node stats from all the satellites
// known to the storagenode and stores this information into db
func (s *Service) CacheStatsFromSatellites(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	satellites, err := s.consoleDB.Satellites().GetIDs(ctx, time.Time{}, time.Now())
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

		s.log.Info(fmt.Sprintf("CacheStats %s: %v", satellite, stats))

		satStats, err := s.consoleDB.Stats().Get(ctx, satellite)
		if err != nil {
			s.log.Error(fmt.Sprintf("SAT %s err: %v", satellite, err))
		}
		s.log.Info(fmt.Sprintf("CacheStats QUERY SAT %s: %v", satellite, satStats))
	}

	return cacheStatsErr.Err()
}

// CacheSpaceUsageFromSatellites queries disk space usage from all the satellites
// known to the storagenode and stores this information into db
func (s *Service) CacheSpaceUsageFromSatellites(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	satellites, err := s.consoleDB.Satellites().GetIDs(ctx, time.Time{}, time.Now())
	if err != nil {
		return NodeStatsServiceErr.Wrap(err)
	}

	// get current month edges
	startDate, endDate := getMonthEdges(time.Now().UTC())

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

		s.log.Info(fmt.Sprintf("CacheSpace %s: %v", satellite, spaceUsages))

		perSat, err := s.consoleDB.DiskSpaceUsages().GetDaily(ctx, satellite, startDate, endDate)
		if err != nil {
			s.log.Error(fmt.Sprintf("SAT %s err: %v", satellite, err))
		}

		for _, ps := range perSat {
			s.log.Info(fmt.Sprintf("CacheSpace QUERY SAT %s", satellite))
			s.log.Info(fmt.Sprintf("CacheSpace QUERY rollupID: %d", ps.RollupID))
			s.log.Info(fmt.Sprintf("CacheSpace QUERY atRest: %f", ps.AtRestTotal))
			s.log.Info(fmt.Sprintf("CacheSpace QUERY timestamp: %s", ps.Timestamp))
		}
	}

	totals, err := s.consoleDB.DiskSpaceUsages().GetDailyTotal(ctx, startDate, endDate)
	if err != nil {
		s.log.Error(fmt.Sprintf("TOTAL err: %v", err))
	}

	s.log.Info(fmt.Sprintf("CacheSpace QUERY TOTAL %v", totals))
	for _, t := range totals {
		s.log.Info(fmt.Sprintf("CacheSpace QUERY atRest: %f", t.AtRestTotal))
		s.log.Info(fmt.Sprintf("CacheSpace QUERY timestamp: %s", t.Timestamp))
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
			RollupID:    pbUsage.RollupId,
			SatelliteID: satelliteID,
			AtRestTotal: pbUsage.AtRestTotal,
			Timestamp:   pbUsage.Timestamp,
		})
	}

	return stamps
}

// Close clear time.Tickers
func (s *Service) Close() error {
	defer mon.Task()(nil)(nil)
	s.statsTicker.Stop()
	s.spaceTicker.Stop()
	return nil
}

// getMonthEdges extract month from the provided date and returns its edges
func getMonthEdges(t time.Time) (time.Time, time.Time) {
	startDate := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	endDate := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, -1, t.Location())
	return startDate, endDate
}
