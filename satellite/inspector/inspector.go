// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"
	"encoding/binary"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()
	// Error wraps errors returned from Server struct methods.
	Error = errs.Class("inspector")
)

// Endpoint for checking object and segment health.
//
// architecture: Endpoint
type Endpoint struct {
	internalpb.DRPCHealthInspectorUnimplementedServer
	log      *zap.Logger
	overlay  *overlay.Service
	metabase *metabase.DB
}

// NewEndpoint will initialize an Endpoint struct.
func NewEndpoint(log *zap.Logger, cache *overlay.Service, metabase *metabase.DB) *Endpoint {
	return &Endpoint{
		log:      log,
		overlay:  cache,
		metabase: metabase,
	}
}

// ObjectHealth will check the health of an object.
func (endpoint *Endpoint) ObjectHealth(ctx context.Context, in *internalpb.ObjectHealthRequest) (resp *internalpb.ObjectHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var segmentHealthResponses []*internalpb.SegmentHealth
	var redundancy *pb.RedundancyScheme

	limit := int(100)
	if in.GetLimit() > 0 {
		limit = int(in.GetLimit())
	}

	var startPosition metabase.SegmentPosition

	if in.GetStartAfterSegment() > 0 {
		startPosition = metabase.SegmentPositionFromEncoded(uint64(in.GetStartAfterSegment()))
	}

	projectID, err := uuid.FromBytes(in.GetProjectId())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	objectLocation := metabase.ObjectLocation{
		ProjectID:  projectID,
		BucketName: string(in.GetBucket()),
		ObjectKey:  metabase.ObjectKey(in.GetEncryptedPath()),
	}

	object, err := endpoint.metabase.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
		ObjectLocation: objectLocation,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	listResult, err := endpoint.metabase.ListSegments(ctx, metabase.ListSegments{
		StreamID: object.StreamID,
		Cursor:   startPosition,
		Limit:    limit,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, segment := range listResult.Segments {
		if !segment.Inline() {
			segmentHealth, err := endpoint.segmentHealth(ctx, segment)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			segmentHealthResponses = append(segmentHealthResponses, segmentHealth.GetHealth())
			redundancy = segmentHealth.GetRedundancy()
		}
	}

	return &internalpb.ObjectHealthResponse{
		Segments:   segmentHealthResponses,
		Redundancy: redundancy,
	}, nil
}

// SegmentHealth will check the health of a segment.
func (endpoint *Endpoint) SegmentHealth(ctx context.Context, in *internalpb.SegmentHealthRequest) (_ *internalpb.SegmentHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	projectID, err := uuid.FromBytes(in.GetProjectId())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	objectLocation := metabase.ObjectLocation{
		ProjectID:  projectID,
		BucketName: string(in.GetBucket()),
		ObjectKey:  metabase.ObjectKey(in.GetEncryptedPath()),
	}

	object, err := endpoint.metabase.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
		ObjectLocation: objectLocation,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	segment, err := endpoint.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: object.StreamID,
		Position: metabase.SegmentPositionFromEncoded(uint64(in.GetSegmentIndex())),
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if segment.Inline() {
		return nil, Error.New("cannot check health of inline segment")
	}

	return endpoint.segmentHealth(ctx, segment)
}

func (endpoint *Endpoint) segmentHealth(ctx context.Context, segment metabase.Segment) (_ *internalpb.SegmentHealthResponse, err error) {

	health := &internalpb.SegmentHealth{}
	var nodeIDs storj.NodeIDList
	for _, piece := range segment.Pieces {
		nodeIDs = append(nodeIDs, piece.StorageNode)
	}

	unreliableOrOfflineNodes, err := endpoint.overlay.KnownUnreliableOrOffline(ctx, nodeIDs)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	offlineNodes, err := endpoint.overlay.KnownOffline(ctx, nodeIDs)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	offlineMap := make(map[storj.NodeID]bool)
	for _, id := range offlineNodes {
		offlineMap[id] = true
	}
	unreliableOfflineMap := make(map[storj.NodeID]bool)
	for _, id := range unreliableOrOfflineNodes {
		unreliableOfflineMap[id] = true
	}

	redundancy := &pb.RedundancyScheme{
		MinReq:           int32(segment.Redundancy.RequiredShares),
		RepairThreshold:  int32(segment.Redundancy.RepairShares),
		SuccessThreshold: int32(segment.Redundancy.OptimalShares),
		Total:            int32(segment.Redundancy.TotalShares),
	}

	var healthyNodes storj.NodeIDList
	var unhealthyNodes storj.NodeIDList
	for _, id := range nodeIDs {
		if offlineMap[id] {
			continue
		}
		if unreliableOfflineMap[id] {
			unhealthyNodes = append(unhealthyNodes, id)
		} else {
			healthyNodes = append(healthyNodes, id)
		}
	}
	health.HealthyIds = healthyNodes
	health.UnhealthyIds = unhealthyNodes
	health.OfflineIds = offlineNodes

	health.Segment = make([]byte, 8)

	binary.LittleEndian.PutUint64(health.Segment, segment.Position.Encode())

	return &internalpb.SegmentHealthResponse{
		Health:     health,
		Redundancy: redundancy,
	}, nil
}
