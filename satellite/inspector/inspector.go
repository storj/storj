// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"
	"strconv"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()
	// Error wraps errors returned from Server struct methods.
	Error = errs.Class("Endpoint error")
)

const lastSegmentIndex = int64(-1)

// Endpoint for checking object and segment health
//
// architecture: Endpoint
type Endpoint struct {
	log      *zap.Logger
	overlay  *overlay.Service
	metainfo *metainfo.Service
}

// NewEndpoint will initialize an Endpoint struct.
func NewEndpoint(log *zap.Logger, cache *overlay.Service, metainfo *metainfo.Service) *Endpoint {
	return &Endpoint{
		log:      log,
		overlay:  cache,
		metainfo: metainfo,
	}
}

// ObjectHealth will check the health of an object.
func (endpoint *Endpoint) ObjectHealth(ctx context.Context, in *internalpb.ObjectHealthRequest) (resp *internalpb.ObjectHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	var segmentHealthResponses []*internalpb.SegmentHealth
	var redundancy *pb.RedundancyScheme

	limit := int64(100)
	if in.GetLimit() > 0 {
		limit = int64(in.GetLimit())
	}

	var start int64
	if in.GetStartAfterSegment() > 0 {
		start = in.GetStartAfterSegment() + 1
	}

	end := limit + start
	if in.GetEndBeforeSegment() > 0 {
		end = in.GetEndBeforeSegment()
	}

	bucket := in.GetBucket()
	encryptedPath := in.GetEncryptedPath()
	projectID := in.GetProjectId()

	segmentIndex := start
	for segmentIndex < end {
		if segmentIndex-start >= limit {
			break
		}

		segment := &internalpb.SegmentHealthRequest{
			Bucket:        bucket,
			EncryptedPath: encryptedPath,
			SegmentIndex:  segmentIndex,
			ProjectId:     projectID,
		}

		segmentHealth, err := endpoint.SegmentHealth(ctx, segment)
		if err != nil {
			if segmentIndex == lastSegmentIndex {
				return nil, Error.Wrap(err)
			}

			segmentIndex = lastSegmentIndex
			continue
		}

		segmentHealthResponses = append(segmentHealthResponses, segmentHealth.GetHealth())
		redundancy = segmentHealth.GetRedundancy()

		if segmentIndex == lastSegmentIndex {
			break
		}

		segmentIndex++
	}

	return &internalpb.ObjectHealthResponse{
		Segments:   segmentHealthResponses,
		Redundancy: redundancy,
	}, nil
}

// SegmentHealth will check the health of a segment.
func (endpoint *Endpoint) SegmentHealth(ctx context.Context, in *internalpb.SegmentHealthRequest) (resp *internalpb.SegmentHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	health := &internalpb.SegmentHealth{}

	projectID, err := uuid.FromString(string(in.GetProjectId()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	location, err := metainfo.CreatePath(ctx, projectID, in.GetSegmentIndex(), in.GetBucket(), in.GetEncryptedPath())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pointer, err := endpoint.metainfo.Get(ctx, location.Encode())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return nil, Error.New("cannot check health of inline segment")
	}

	var nodeIDs storj.NodeIDList
	for _, piece := range pointer.GetRemote().GetRemotePieces() {
		nodeIDs = append(nodeIDs, piece.NodeId)
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

	if in.GetSegmentIndex() > -1 {
		health.Segment = []byte("s" + strconv.FormatInt(in.GetSegmentIndex(), 10))
	} else {
		health.Segment = []byte("l")
	}

	return &internalpb.SegmentHealthResponse{
		Health:     health,
		Redundancy: pointer.GetRemote().GetRedundancy(),
	}, nil
}
