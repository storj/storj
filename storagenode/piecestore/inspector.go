// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
)

// Inspector does inspectory things
type Inspector struct {
	log       *zap.Logger
	pieceInfo pieces.DB
	kademlia  *kademlia.Kademlia
	usageDB   bandwidth.DB
	psdbDB    *psdb.DB // TODO remove after complete migration

	startTime time.Time
	config    psserver.Config
}

// NewInspector creates piecestore inspector instance
func NewInspector(log *zap.Logger, pieceInfo pieces.DB, kademlia *kademlia.Kademlia, usageDB bandwidth.DB, psdbDB *psdb.DB, config psserver.Config) *Inspector {
	return &Inspector{
		log:       log,
		pieceInfo: pieceInfo,
		kademlia:  kademlia,
		usageDB:   usageDB,
		psdbDB:    psdbDB,
		config:    config,
		startTime: time.Now(),
	}
}

func (inspector *Inspector) retrieveStats(ctx context.Context) (*pb.StatSummaryResponse, error) {
	totalUsedSpace, err := inspector.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}
	usage, err := inspector.usageDB.Summary(ctx, getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, err
	}
	totalUsedBandwidth := int64(0)
	oldUsage, err := inspector.psdbDB.SumTTLSizes()
	if err != nil {
		inspector.log.Warn("unable to calculate old bandwidth usage")
	} else {
		totalUsedBandwidth = oldUsage
	}

	totalUsedBandwidth += usage.Total()

	return &pb.StatSummaryResponse{
		UsedSpace:          totalUsedSpace,
		AvailableSpace:     (inspector.config.AllocatedDiskSpace.Int64() - totalUsedSpace),
		UsedBandwidth:      totalUsedBandwidth,
		AvailableBandwidth: (inspector.config.AllocatedBandwidth.Int64() - totalUsedBandwidth),
	}, nil
}

// Stats returns current statistics about the storage node
func (inspector *Inspector) Stats(ctx context.Context, in *pb.StatsRequest) (*pb.StatSummaryResponse, error) {
	inspector.log.Debug("Getting Stats...")

	statsSummary, err := inspector.retrieveStats(ctx)
	if err != nil {
		return nil, err
	}

	inspector.log.Info("Successfully retrieved Stats...")

	return statsSummary, nil
}

func (inspector *Inspector) getDashboardData(ctx context.Context) (*pb.DashboardResponse, error) {
	statsSummary, err := inspector.retrieveStats(ctx)
	if err != nil {
		return &pb.DashboardResponse{}, Error.Wrap(err)
	}

	// TODO: querying all nodes is slow, find a more performant way to do this.
	nodes, err := inspector.kademlia.FindNear(ctx, storj.NodeID{}, 10000000)
	if err != nil {
		return &pb.DashboardResponse{}, Error.Wrap(err)
	}

	bootstrapNodes := inspector.kademlia.GetBootstrapNodes()
	bsNodes := make([]string, len(bootstrapNodes))
	for i, node := range bootstrapNodes {
		bsNodes[i] = node.Address.Address
	}

	return &pb.DashboardResponse{
		NodeId:           inspector.kademlia.Local().Id,
		NodeConnections:  int64(len(nodes)),
		BootstrapAddress: strings.Join(bsNodes[:], ", "),
		InternalAddress:  "",
		ExternalAddress:  inspector.kademlia.Local().Address.Address,
		Connection:       true,
		Uptime:           ptypes.DurationProto(time.Since(inspector.startTime)),
		Stats:            statsSummary,
	}, nil
}

// Dashboard returns dashboard information
func (inspector *Inspector) Dashboard(ctx context.Context, in *pb.DashboardRequest) (*pb.DashboardResponse, error) {
	data, err := inspector.getDashboardData(ctx)
	if err != nil {
		inspector.log.Warn("unable to get dashboard information")
		return nil, err
	}
	return data, nil
}

func getBeginningOfMonth() time.Time {
	t := time.Now()
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().Location())
}
