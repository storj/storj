// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
)

// Repairer for segments
type Repairer struct {
	oc        overlay.Client
	ec        ecclient.Client
	pdb       pdbclient.Client
	nodeStats *pb.NodeStats
}

// NewSegmentRepairer creates a new instance of SegmentRepairer
func NewSegmentRepairer(oc overlay.Client, ec ecclient.Client, pdb pdbclient.Client) *Repairer {
	return &Repairer{oc: oc, ec: ec, pdb: pdb}
}

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
func (s *Repairer) Repair(ctx context.Context, path storj.Path, lostPieces []int32) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Read the segment's pointer's info from the PointerDB
	pr, originalNodes, _, err := s.pdb.Get(ctx, path)
	if err != nil {
		return Error.Wrap(err)
	}

	if pr.GetType() != pb.Pointer_REMOTE {
		return Error.New("cannot repair inline segment %s", psclient.PieceID(pr.GetInlineSegment()))
	}

	seg := pr.GetRemote()
	pid := psclient.PieceID(seg.GetPieceId())

	originalNodes, err = lookupAndAlignNodes(ctx, s.oc, originalNodes, seg)
	if err != nil {
		return Error.Wrap(err)
	}

	// Get the nodes list that needs to be excluded
	var excludeNodeIDs storj.NodeIDList

	// Count the number of nil nodes thats needs to be repaired
	totalNilNodes := 0

	healthyNodes := make([]*pb.Node, len(originalNodes))

	// Populate healthyNodes with all nodes from originalNodes except those correlating to indices in lostPieces
	for i, v := range originalNodes {
		if v == nil {
			totalNilNodes++
			continue
		}
		v.Type.DPanicOnInvalid("repair")
		excludeNodeIDs = append(excludeNodeIDs, v.Id)

		// If node index exists in lostPieces, skip adding it to healthyNodes
		if contains(lostPieces, i) {
			totalNilNodes++
		} else {
			healthyNodes[i] = v
		}
	}

	// Request Overlay for n-h new storage nodes
	op := overlay.Options{Amount: totalNilNodes, Space: 0, Excluded: excludeNodeIDs}
	newNodes, err := s.oc.Choose(ctx, op)
	if err != nil {
		return err
	}

	if totalNilNodes != len(newNodes) {
		return Error.New("Number of new nodes from overlay (%d) does not equal total nil nodes (%d)", len(newNodes), totalNilNodes)
	}

	totalRepairCount := len(newNodes)

	// Make a repair nodes list just with new unique ids
	repairNodes := make([]*pb.Node, len(healthyNodes))
	for i, vr := range healthyNodes {
		// Check that totalRepairCount is non-negative
		if totalRepairCount < 0 {
			return Error.New("Total repair count (%d) less than zero", totalRepairCount)
		}

		// Find the nil nodes in the healthyNodes list
		if vr == nil {
			// Assign the item in repairNodes list with an item from the newNode list
			totalRepairCount--
			repairNodes[i] = newNodes[totalRepairCount]
		}
	}
	for _, v := range repairNodes {
		if v != nil {
			v.Type.DPanicOnInvalid("repair 2")
		}
	}

	// Check that all nil nodes have a replacement prepared
	if totalRepairCount != 0 {
		return Error.New("Failed to replace all nil nodes (%d). (%d) new nodes not inserted", len(newNodes), totalRepairCount)
	}

	rs, err := makeRedundancyStrategy(pr.GetRemote().GetRedundancy())
	if err != nil {
		return Error.Wrap(err)
	}

	pbaGet, err := s.pdb.PayerBandwidthAllocation(ctx, pb.BandwidthAction_GET_REPAIR)
	if err != nil {
		return Error.Wrap(err)
	}
	// Download the segment using just the healthyNodes
	rr, err := s.ec.Get(ctx, healthyNodes, rs, pid, pr.GetSegmentSize(), pbaGet, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	r, err := rr.Range(ctx, 0, rr.Size())
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, r.Close()) }()

	pbaPut, err := s.pdb.PayerBandwidthAllocation(ctx, pb.BandwidthAction_PUT_REPAIR)
	if err != nil {
		return Error.Wrap(err)
	}
	// Upload the repaired pieces to the repairNodes
	successfulNodes, err := s.ec.Put(ctx, repairNodes, rs, pid, r, convertTime(pr.GetExpirationDate()), pbaPut, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	// Merge the successful nodes list into the healthy nodes list
	for i, v := range healthyNodes {
		if v == nil {
			// copy the successfuNode info
			healthyNodes[i] = successfulNodes[i]
		}
	}

	metadata := pr.GetMetadata()
	pointer, err := makeRemotePointer(healthyNodes, rs, pid, rr.Size(), pr.GetExpirationDate(), metadata)
	if err != nil {
		return err
	}

	// update the segment info in the pointerDB
	return s.pdb.Put(ctx, path, pointer)
}
