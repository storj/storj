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

func (s *Inspector) retrieveStats() (*pb.StatSummary, error) {
	totalUsed, err := s.ps.DB.SumTTLSizes()
	if err != nil {
		return nil, err
	}

	totalUsedBandwidth, err := s.ps.DB.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, err
	}

	return &pb.StatSummary{
		UsedSpace:          totalUsed,
		AvailableSpace:     (s.ps.totalAllocated - totalUsed),
		UsedBandwidth:      totalUsedBandwidth,
		AvailableBandwidth: (s.ps.totalBwAllocated - totalUsedBandwidth),
	}, nil
}

// Stats returns current statistics about the server.
func (s *Inspector) Stats(ctx context.Context, in *pb.StatsReq) (*pb.StatSummary, error) {
	s.ps.log.Debug("Getting Stats...")

	statsSummary, err := s.retrieveStats()
	if err != nil {
		return nil, err
	}

	s.ps.log.Info("Successfully retrieved Stats...")

	return statsSummary, nil
}

func (s *Inspector) getDashboardData(ctx context.Context) (*pb.DashboardStats, error) {
	statsSummary, err := s.retrieveStats()
	if err != nil {
		return &pb.DashboardStats{}, ServerError.Wrap(err)
	}

	nodes, err := s.ps.kad.FindNear(ctx, storj.NodeID{}, 10000000)
	if err != nil {
		return &pb.DashboardStats{}, ServerError.Wrap(err)
	}

	bootstrapNodes := s.ps.kad.GetBootstrapNodes()

	bsNodes := make([]string, len(bootstrapNodes))

	for i, node := range bootstrapNodes {
		bsNodes[i] = node.Address.Address
	}

	return &pb.DashboardStats{
		NodeId:           s.ps.kad.Local().Id.String(),
		NodeConnections:  int64(len(nodes)),
		BootstrapAddress: strings.Join(bsNodes[:], ", "),
		InternalAddress:  "",
		ExternalAddress:  s.ps.kad.Local().Address.Address,
		Connection:       true,
		Uptime:           ptypes.DurationProto(time.Since(s.ps.startTime)),
		Stats:            statsSummary,
	}, nil
}

// Dashboard is a stream that sends data every `interval` seconds to the listener.
func (s *Inspector) Dashboard(in *pb.DashboardReq, stream pb.PieceStoreRoutes_DashboardServer) (err error) {
	ctx := stream.Context()
	ticker := time.NewTicker(3 * time.Second)

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return nil
			}

			return ctx.Err()
		case <-ticker.C:
			data, err := s.getDashboardData(ctx)
			if err != nil {
				s.ps.log.Warn("unable to create dashboard data proto")
				continue
			}

			if err := stream.Send(data); err != nil {
				s.ps.log.Error("error sending dashboard stream", zap.Error(err))
				return err
			}
		}
	}
}
