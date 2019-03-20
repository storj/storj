// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Inspector is a gRPC service for inspecting psserver internals
type Inspector struct {
	ps *Server
}

// NewInspector creates an Inspector
func NewInspector(psserver *Server) *Inspector {
	return &Inspector{
		ps: psserver,
	}
}

func (s *Inspector) retrieveStats() (*pb.StatSummaryResponse, error) {
	totalUsed, err := s.ps.DB.SumTTLSizes()
	if err != nil {
		return nil, err
	}

	totalUsedBandwidth, err := s.ps.DB.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, err
	}

	return &pb.StatSummaryResponse{
		UsedSpace:          totalUsed,
		AvailableSpace:     (s.ps.totalAllocated - totalUsed),
		UsedBandwidth:      totalUsedBandwidth,
		AvailableBandwidth: (s.ps.totalBwAllocated - totalUsedBandwidth),
	}, nil
}

// Stats returns current statistics about the server.
func (s *Inspector) Stats(ctx context.Context, in *pb.StatsRequest) (*pb.StatSummaryResponse, error) {
	s.ps.log.Debug("Getting Stats...")

	statsSummary, err := s.retrieveStats()
	if err != nil {
		return nil, err
	}

	s.ps.log.Info("Successfully retrieved Stats...")

	return statsSummary, nil
}

func (s *Inspector) getDashboardData(ctx context.Context) (*pb.DashboardResponse, error) {
	statsSummary, err := s.retrieveStats()
	if err != nil {
		return &pb.DashboardResponse{}, ServerError.Wrap(err)
	}

	nodes, err := s.ps.kad.FindNear(ctx, storj.NodeID{}, 10000000)
	if err != nil {
		return &pb.DashboardResponse{}, ServerError.Wrap(err)
	}

	bootstrapNodes := s.ps.kad.GetBootstrapNodes()

	bsNodes := make([]string, len(bootstrapNodes))

	for i, node := range bootstrapNodes {
		bsNodes[i] = node.Address.Address
	}

	pinged, err := ptypes.TimestampProto(s.ps.kad.LastPinged())
	if err != nil {
		s.ps.log.Warn("last ping time bad", zap.Error(err))
		pinged = nil
	}
	queried, err := ptypes.TimestampProto(s.ps.kad.LastQueried())
	if err != nil {
		s.ps.log.Warn("last query time bad", zap.Error(err))
		queried = nil
	}

	return &pb.DashboardResponse{
		NodeId:           s.ps.kad.Local().Id.String(),
		NodeConnections:  int64(len(nodes)),
		BootstrapAddress: strings.Join(bsNodes[:], ", "),
		InternalAddress:  "",
		ExternalAddress:  s.ps.kad.Local().Address.Address,
		LastPinged:       pinged,
		LastQueried:      queried,
		Uptime:           ptypes.DurationProto(time.Since(s.ps.startTime)),
		Stats:            statsSummary,
	}, nil
}

// Dashboard returns dashboard data.
func (s *Inspector) Dashboard(ctx context.Context, in *pb.DashboardRequest) (*pb.DashboardResponse, error) {
	data, err := s.getDashboardData(ctx)
	if err != nil {
		s.ps.log.Warn("unable to create dashboard data proto")
		return nil, err
	}
	return data, nil
}
