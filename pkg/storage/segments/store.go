// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/vivint/infectious"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/ranger"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/metainfo"
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
	metainfo       metainfo.Client
	oc             overlay.Client
	ec             ecclient.Client
	pdb            pdbclient.Client
	rs             eestream.RedundancyStrategy
	thresholdSize  int
	maxSegmentSize int64
}

// NewSegmentStore creates a new instance of segmentStore
func NewSegmentStore(metainfo metainfo.Client, oc overlay.Client, ec ecclient.Client, pdb pdbclient.Client, rs eestream.RedundancyStrategy, threshold int) Store {
	return &segmentStore{
		metainfo:      metainfo,
		oc:            oc, // TODO: remove
		ec:            ec,
		pdb:           pdb, // TODO: remove
		rs:            rs,
		thresholdSize: threshold,
	}
}

// Meta retrieves the metadata of the segment
func (s *segmentStore) Meta(ctx context.Context, path storj.Path) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, objectPath, segmentIndex, err := split(path)
	if err != nil {
		return Meta{}, err
	}

	pointer, err := s.metainfo.SegmentInfo(ctx, bucket, objectPath, segmentIndex)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	return convertMeta(pointer), nil
}

// Put uploads a segment to an erasure code client
func (s *segmentStore) Put(ctx context.Context, data io.Reader, expiration time.Time, segmentInfo func() (storj.Path, []byte, error)) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	redundancy := &pb.RedundancyScheme{
		Type:             pb.RedundancyScheme_RS,
		MinReq:           int32(s.rs.RequiredCount()),
		Total:            int32(s.rs.TotalCount()),
		RepairThreshold:  int32(s.rs.RepairThreshold()),
		SuccessThreshold: int32(s.rs.OptimalThreshold()),
		ErasureShareSize: int32(s.rs.ErasureShareSize()),
	}

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
	var originalLimits []*pb.OrderLimit2
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
		limits, rootPieceID, err := s.metainfo.CreateSegment(ctx, "", "", -1, redundancy, s.maxSegmentSize, expiration) // bucket, path and segment index are not known at this point
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}

		sizedReader := SizeReader(peekReader)

		successfulNodes, successfulHashes, err := s.ec.Put(ctx, limits, s.rs, sizedReader, expiration)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}

		p, metadata, err := segmentInfo()
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		path = p

		pointer, err = makeRemotePointer(successfulNodes, successfulHashes, s.rs, rootPieceID.String(), sizedReader.Size(), exp, metadata)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}

		originalLimits := make([]*pb.OrderLimit2, len(limits))
		for i, addressedLimit := range limits {
			originalLimits[i] = addressedLimit.GetLimit()
		}
	}

	bucket, objectPath, segmentIndex, err := split(path)
	if err != nil {
		return Meta{}, err
	}

	savedPointer, err := s.metainfo.CommitSegment(ctx, bucket, objectPath, segmentIndex, pointer, originalLimits)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	return convertMeta(savedPointer), nil
}

// Get requests the satellite to read a segment and downloaded the pieces from the storage nodes
func (s *segmentStore) Get(ctx context.Context, path storj.Path) (rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, objectPath, segmentIndex, err := split(path)
	if err != nil {
		return nil, Meta{}, err
	}

	pointer, limits, err := s.metainfo.ReadSegment(ctx, bucket, objectPath, segmentIndex)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	switch pointer.GetType() {
	case pb.Pointer_INLINE:
		return ranger.ByteRanger(pointer.InlineSegment), convertMeta(pointer), nil
	case pb.Pointer_REMOTE:
		needed := calcNeededNodes(pointer.GetRemote().GetRedundancy())
		selected := make([]*pb.AddressedOrderLimit, len(limits))

		for _, i := range rand.Perm(len(limits)) {
			limit := limits[i]
			if limit == nil {
				continue
			}

			selected[i] = limit

			needed--
			if needed <= 0 {
				break
			}
		}

		redundancy, err := makeRedundancyStrategy(pointer.GetRemote().GetRedundancy())
		if err != nil {
			return nil, Meta{}, err
		}

		rr, err = s.ec.Get(ctx, selected, redundancy, pointer.GetSegmentSize())
		if err != nil {
			return nil, Meta{}, Error.Wrap(err)
		}

		return rr, convertMeta(pointer), nil
	default:
		return nil, Meta{}, Error.New("unsupported pointer type: %d", pointer.GetType())
	}
}

// makeRemotePointer creates a pointer of type remote
func makeRemotePointer(nodes []*pb.Node, hashes []*pb.SignedHash, rs eestream.RedundancyStrategy, pieceID string, readerSize int64, exp *timestamp.Timestamp, metadata []byte) (pointer *pb.Pointer, err error) {
	if len(nodes) != len(hashes) {
		return nil, Error.New("unable to make pointer: size of nodes != size of hashes")
	}

	var remotePieces []*pb.RemotePiece
	for i := range nodes {
		if nodes[i] == nil {
			continue
		}
		nodes[i].Type.DPanicOnInvalid("makeremotepointer")
		remotePieces = append(remotePieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   nodes[i].Id,
			Hash:     hashes[i],
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
			PieceId:      pieceID,
			RemotePieces: remotePieces,
		},
		SegmentSize:    readerSize,
		ExpirationDate: exp,
		Metadata:       metadata,
	}
	return pointer, nil
}

// Delete requests the satellite to delete a segment and tells storage nodes
// to delete the segment's pieces.
func (s *segmentStore) Delete(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, objectPath, segmentIndex, err := split(path)
	if err != nil {
		return err
	}

	limits, err := s.metainfo.DeleteSegment(ctx, bucket, objectPath, segmentIndex)
	if err != nil {
		return Error.Wrap(err)
	}

	if len(limits) == 0 {
		// inline segment - nothing else to do
		return
	}

	// remote segment - delete the pieces from storage nodes
	err = s.ec.Delete(ctx, limits)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// List retrieves paths to segments and their metadata stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, prefix, startAfter, endBefore storj.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	bucket, strippedPrefix, _, err := split(prefix)
	if err != nil {
		return nil, false, Error.Wrap(err)
	}

	list, more, err := s.metainfo.ListSegments(ctx, bucket, strippedPrefix, startAfter, endBefore, recursive, int32(limit), metaFlags)
	if err != nil {
		return nil, false, Error.Wrap(err)
	}

	items = make([]ListItem, len(list))
	for i, itm := range list {
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

func split(path storj.Path) (bucket string, objectPath storj.Path, segmentIndex int64, err error) {
	components := storj.SplitPath(path)
	if len(components) < 1 {
		return "", "", -2, Error.New("empty path", len(components))
	}

	segmentIndex, err = convertSegmentIndex(components[0])
	if err != nil {
		return "", "", -2, err
	}

	if len(components) > 1 {
		bucket = components[1]
		objectPath = storj.JoinPaths(components[2:]...)
	}

	return bucket, objectPath, segmentIndex, nil
}

func convertSegmentIndex(segmentComp string) (segmentIndex int64, err error) {
	switch {
	case segmentComp == "l":
		return -1, nil
	case strings.HasPrefix(segmentComp, "s"):
		num, err := strconv.Atoi(segmentComp[1:])
		if err != nil {
			return -2, Error.Wrap(err)
		}
		return int64(num), nil
	default:
		return -2, Error.New("invalid segment component: %s", segmentComp)
	}
}
