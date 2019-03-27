// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore monitor errors
	Error = errs.Class("piecestore inspector")
)

// Endpoint does inspectory things
type Endpoint struct {
	log       *zap.Logger
	pieceInfo pieces.DB
	kademlia  *kademlia.Kademlia
	usageDB   bandwidth.DB
	psdbDB    *psdb.DB // TODO remove after complete migration

	startTime time.Time
	config    psserver.Config
}

// NewEndpoint creates piecestore inspector instance
func NewEndpoint(log *zap.Logger, pieceInfo pieces.DB, kademlia *kademlia.Kademlia, usageDB bandwidth.DB, psdbDB *psdb.DB, config psserver.Config) *Endpoint {
	return &Endpoint{
		log:       log,
		pieceInfo: pieceInfo,
		kademlia:  kademlia,
		usageDB:   usageDB,
		psdbDB:    psdbDB,
		config:    config,
		startTime: time.Now(),
	}
}

func (inspector *Endpoint) retrieveStats(ctx context.Context) (*pb.StatSummaryResponse, error) {
	totalUsedSpace, err := inspector.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}
	usage, err := inspector.usageDB.Summary(ctx, getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, err
	}
	ingress := usage.Put + usage.PutRepair
	egress := usage.Get + usage.GetAudit + usage.GetRepair

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
		UsedIngress:        ingress,
		UsedEgress:         egress,
		UsedBandwidth:      totalUsedBandwidth,
		AvailableBandwidth: (inspector.config.AllocatedBandwidth.Int64() - totalUsedBandwidth),
	}, nil
}

// Stats returns current statistics about the storage node
func (inspector *Endpoint) Stats(ctx context.Context, in *pb.StatsRequest) (out *pb.StatSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	inspector.log.Debug("Getting Stats...")

	statsSummary, err := inspector.retrieveStats(ctx)
	if err != nil {
		return nil, err
	}

	inspector.log.Info("Successfully retrieved Stats...")

	return statsSummary, nil
}

func (inspector *Endpoint) getDashboardData(ctx context.Context) (*pb.DashboardResponse, error) {
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

	pinged, err := ptypes.TimestampProto(inspector.kademlia.LastPinged())
	if err != nil {
		inspector.log.Warn("last ping time bad", zap.Error(err))
		pinged = nil
	}
	queried, err := ptypes.TimestampProto(inspector.kademlia.LastQueried())
	if err != nil {
		inspector.log.Warn("last query time bad", zap.Error(err))
		queried = nil
	}

	return &pb.DashboardResponse{
		NodeId:           inspector.kademlia.Local().Id,
		NodeConnections:  int64(len(nodes)),
		BootstrapAddress: strings.Join(bsNodes[:], ", "),
		InternalAddress:  "",
		ExternalAddress:  inspector.kademlia.Local().Address.Address,
		LastPinged:       pinged,
		LastQueried:      queried,
		Uptime:           ptypes.DurationProto(time.Since(inspector.startTime)),
		Stats:            statsSummary,
	}, nil
}

// Dashboard returns dashboard information
func (inspector *Endpoint) Dashboard(ctx context.Context, in *pb.DashboardRequest) (out *pb.DashboardResponse, err error) {
	defer mon.Task()(&ctx)(&err)

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
