// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"io"
	"math/rand"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/vivint/infectious"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/ranger"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
)

// Meta info about a segment
type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Size       int64
	Data       []byte
}

// ListItem is a single item in a listing
type ListItem struct {
	Path     storj.Path
	Meta     Meta
	IsPrefix bool
}

// Store for segments
type Store interface {
	Meta(ctx context.Context, path storj.Path) (meta Meta, err error)
	Get(ctx context.Context, path storj.Path) (rr ranger.Ranger, meta Meta, err error)
	Put(ctx context.Context, data io.Reader, expiration time.Time, segmentInfo func() (storj.Path, []byte, error)) (meta Meta, err error)
	Delete(ctx context.Context, path storj.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

type segmentStore struct {
	oc            overlay.Client
	ec            ecclient.Client
	pdb           pdbclient.Client
	rs            eestream.RedundancyStrategy
	thresholdSize int
}

// NewSegmentStore creates a new instance of segmentStore
func NewSegmentStore(oc overlay.Client, ec ecclient.Client, pdb pdbclient.Client, rs eestream.RedundancyStrategy, threshold int) Store {
	return &segmentStore{
		oc:            oc,
		ec:            ec,
		pdb:           pdb,
		rs:            rs,
		thresholdSize: threshold,
	}
}

// Meta retrieves the metadata of the segment
func (s *segmentStore) Meta(ctx context.Context, path storj.Path) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	pr, _, _, err := s.pdb.Get(ctx, path)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	return convertMeta(pr), nil
}

// Put uploads a segment to an erasure code client
func (s *segmentStore) Put(ctx context.Context, data io.Reader, expiration time.Time, segmentInfo func() (storj.Path, []byte, error)) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	exp, err := ptypes.TimestampProto(expiration)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	peekReader := NewPeekThresholdReader(data)
	remoteSized, err := peekReader.IsLargerThan(s.thresholdSize)
	if err != nil {
		return Meta{}, err
	}

	var path storj.Path
	var pointer *pb.Pointer
	if !remoteSized {
		p, metadata, err := segmentInfo()
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		path = p

		pointer = &pb.Pointer{
			Type:           pb.Pointer_INLINE,
			InlineSegment:  peekReader.thresholdBuf,
			SegmentSize:    int64(len(peekReader.thresholdBuf)),
			ExpirationDate: exp,
			Metadata:       metadata,
		}
	} else {
		sizedReader := SizeReader(peekReader)

		// uses overlay client to request a list of nodes according to configured standards
		nodes, err := s.oc.Choose(ctx,
			overlay.Options{
				Amount:    s.rs.TotalCount(),
				Bandwidth: sizedReader.Size() / int64(s.rs.TotalCount()),
				Space:     sizedReader.Size() / int64(s.rs.TotalCount()),
				Excluded:  nil,
			})
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		for _, v := range nodes {
			if v != nil {
				v.Type.DPanicOnInvalid("ss put")
			}
		}

		pieceID := psclient.NewPieceID()

		pba, err := s.pdb.PayerBandwidthAllocation(ctx, pb.BandwidthAction_PUT)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}

		successfulNodes, err := s.ec.Put(ctx, nodes, s.rs, pieceID, sizedReader, expiration, pba, nil)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}

		p, metadata, err := segmentInfo()
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		path = p

		pointer, err = makeRemotePointer(successfulNodes, s.rs, pieceID, sizedReader.Size(), exp, metadata)
		if err != nil {
			return Meta{}, err
		}
	}

	// puts pointer to pointerDB
	err = s.pdb.Put(ctx, path, pointer)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	// get the metadata for the newly uploaded segment
	m, err := s.Meta(ctx, path)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}
	return m, nil
}

// Get retrieves a segment using erasure code, overlay, and pointerdb clients
func (s *segmentStore) Get(ctx context.Context, path storj.Path) (rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	pr, nodes, pba, err := s.pdb.Get(ctx, path)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	switch pr.GetType() {
	case pb.Pointer_INLINE:
		rr = ranger.ByteRanger(pr.InlineSegment)
	case pb.Pointer_REMOTE:
		seg := pr.GetRemote()
		pid := psclient.PieceID(seg.GetPieceId())

		nodes, err = lookupAndAlignNodes(ctx, s.oc, nodes, seg)
		if err != nil {
			return nil, Meta{}, Error.Wrap(err)
		}

		rs, err := makeRedundancyStrategy(pr.GetRemote().GetRedundancy())
		if err != nil {
			return nil, Meta{}, err
		}

		needed := calcNeededNodes(pr.GetRemote().GetRedundancy())
		selected := make([]*pb.Node, rs.TotalCount())

		for _, i := range rand.Perm(len(nodes)) {
			node := nodes[i]
			if node == nil {
				continue
			}

			selected[i] = node

			needed--
			if needed <= 0 {
				break
			}
			node.Type.DPanicOnInvalid("ss get")
		}

		rr, err = s.ec.Get(ctx, selected, rs, pid, pr.GetSegmentSize(), pba, nil)
		if err != nil {
			return nil, Meta{}, Error.Wrap(err)
		}
	default:
		return nil, Meta{}, Error.New("unsupported pointer type: %d", pr.GetType())
	}

	return rr, convertMeta(pr), nil
}

// makeRemotePointer creates a pointer of type remote
func makeRemotePointer(nodes []*pb.Node, rs eestream.RedundancyStrategy, pieceID psclient.PieceID, readerSize int64, exp *timestamp.Timestamp, metadata []byte) (pointer *pb.Pointer, err error) {
	var remotePieces []*pb.RemotePiece
	for i := range nodes {
		if nodes[i] == nil {
			continue
		}
		nodes[i].Type.DPanicOnInvalid("makeremotepointer")
		remotePieces = append(remotePieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   nodes[i].Id,
		})
	}

	pointer = &pb.Pointer{
		Type: pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			Redundancy: &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_RS,
				MinReq:           int32(rs.RequiredCount()),
				Total:            int32(rs.TotalCount()),
				RepairThreshold:  int32(rs.RepairThreshold()),
				SuccessThreshold: int32(rs.OptimalThreshold()),
				ErasureShareSize: int32(rs.ErasureShareSize()),
			},
			PieceId:      string(pieceID),
			RemotePieces: remotePieces,
		},
		SegmentSize:    readerSize,
		ExpirationDate: exp,
		Metadata:       metadata,
	}
	return pointer, nil
}

// Delete tells piece stores to delete a segment and deletes pointer from pointerdb
func (s *segmentStore) Delete(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	pr, nodes, _, err := s.pdb.Get(ctx, path)
	if err != nil {
		return Error.Wrap(err)
	}

	if pr.GetType() == pb.Pointer_REMOTE {
		seg := pr.GetRemote()
		pid := psclient.PieceID(seg.PieceId)

		nodes, err = lookupAndAlignNodes(ctx, s.oc, nodes, seg)
		if err != nil {
			return Error.Wrap(err)
		}
		for _, v := range nodes {
			if v != nil {
				v.Type.DPanicOnInvalid("ss delete")
			}
		}

		// ecclient sends delete request
		err = s.ec.Delete(ctx, nodes, pid, nil)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	// deletes pointer from pointerdb
	return s.pdb.Delete(ctx, path)
}

// List retrieves paths to segments and their metadata stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	pdbItems, more, err := s.pdb.List(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(pdbItems))
	for i, itm := range pdbItems {
		items[i] = ListItem{
			Path:     itm.Path,
			Meta:     convertMeta(itm.Pointer),
			IsPrefix: itm.IsPrefix,
		}
	}

	return items, more, nil
}

func makeRedundancyStrategy(scheme *pb.RedundancyScheme) (eestream.RedundancyStrategy, error) {
	fc, err := infectious.NewFEC(int(scheme.GetMinReq()), int(scheme.GetTotal()))
	if err != nil {
		return eestream.RedundancyStrategy{}, Error.Wrap(err)
	}
	es := eestream.NewRSScheme(fc, int(scheme.GetErasureShareSize()))
	return eestream.NewRedundancyStrategy(es, int(scheme.GetRepairThreshold()), int(scheme.GetSuccessThreshold()))
}

// calcNeededNodes calculate how many minimum nodes are needed for download,
// based on t = k + (n-o)k/o
func calcNeededNodes(rs *pb.RedundancyScheme) int32 {
	extra := int32(1)

	if rs.GetSuccessThreshold() > 0 {
		extra = ((rs.GetTotal() - rs.GetSuccessThreshold()) * rs.GetMinReq()) / rs.GetSuccessThreshold()
		if extra == 0 {
			// ensure there is at least one extra node, so we can have error detection/correction
			extra = 1
		}
	}

	needed := rs.GetMinReq() + extra

	if needed > rs.GetTotal() {
		needed = rs.GetTotal()
	}

	return needed
}

// lookupNodes, if necessary, calls Lookup to get node addresses from the overlay.
// It also realigns the nodes to an indexed list of nodes based on the piece number.
// Missing pieces are represented by a nil node.
func lookupAndAlignNodes(ctx context.Context, oc overlay.Client, nodes []*pb.Node, seg *pb.RemoteSegment) (result []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if nodes == nil {
		// Get list of all nodes IDs storing a piece from the segment
		var nodeIds storj.NodeIDList
		for _, p := range seg.RemotePieces {
			nodeIds = append(nodeIds, p.NodeId)
		}
		// Lookup the node info from node IDs
		nodes, err = oc.BulkLookup(ctx, nodeIds)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	for _, v := range nodes {
		if v != nil {
			v.Type.DPanicOnInvalid("lookup and align nodes")
		}
	}

	// Realign the nodes
	result = make([]*pb.Node, seg.GetRedundancy().GetTotal())
	for i, p := range seg.GetRemotePieces() {
		result[p.PieceNum] = nodes[i]
	}

	return result, nil
}

// contains checks if n exists in list
func contains(list []int32, n int) bool {
	for i := range list {
		if n == int(list[i]) {
			return true
		}
	}
	return false
}

// convertMeta converts pointer to segment metadata
func convertMeta(pr *pb.Pointer) Meta {
	return Meta{
		Modified:   convertTime(pr.GetCreationDate()),
		Expiration: convertTime(pr.GetExpirationDate()),
		Size:       pr.GetSegmentSize(),
		Data:       pr.GetMetadata(),
	}
}

// convertTime converts gRPC timestamp to Go time
func convertTime(ts *timestamp.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	t, err := ptypes.Timestamp(ts)
	if err != nil {
		zap.S().Warnf("Failed converting timestamp %v: %v", ts, err)
	}
	return t
}
