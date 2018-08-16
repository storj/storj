// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"io"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/vivint/infectious"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/ec"
	opb "storj.io/storj/protos/overlay"
	ppb "storj.io/storj/protos/pointerdb"
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
	Path paths.Path
	Meta Meta
}

// Store for segments
type Store interface {
	Meta(ctx context.Context, path paths.Path) (meta Meta, err error)
	Get(ctx context.Context, path paths.Path) (rr ranger.RangeCloser,
		meta Meta, err error)
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
		expiration time.Time) (meta Meta, err error)
	Delete(ctx context.Context, path paths.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
		recursive bool, limit int, metaFlags uint32) (items []ListItem,
		more bool, err error)
}

type segmentStore struct {
	oc            overlay.Client
	ec            ecclient.Client
	pdb           pointerdb.Client
	rs            eestream.RedundancyStrategy
	thresholdSize int
}

// NewSegmentStore creates a new instance of segmentStore
func NewSegmentStore(oc overlay.Client, ec ecclient.Client,
	pdb pointerdb.Client, rs eestream.RedundancyStrategy, t int) Store {
	return &segmentStore{oc: oc, ec: ec, pdb: pdb, rs: rs, thresholdSize: t}
}

// Meta retrieves the metadata of the segment
func (s *segmentStore) Meta(ctx context.Context, path paths.Path) (meta Meta,
	err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	return convertMeta(pr), nil
}

// Put uploads a segment to an erasure code client
func (s *segmentStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	var p *ppb.Pointer

	exp, err := ptypes.TimestampProto(expiration)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	peekReader := NewPeekThresholdReader(data)
	remoteSized, err := peekReader.IsLargerThan(s.thresholdSize)
	if err != nil {
		return Meta{}, err
	}
	if !remoteSized {
		p = &ppb.Pointer{
			Type:           ppb.Pointer_INLINE,
			InlineSegment:  peekReader.thresholdBuf,
			Size:           int64(len(peekReader.thresholdBuf)),
			ExpirationDate: exp,
			Metadata:       metadata,
		}
	} else {
		// uses overlay client to request a list of nodes
		nodes, err := s.oc.Choose(ctx, s.rs.TotalCount(), 0)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		pieceID := client.NewPieceID()
		sizedReader := SizeReader(peekReader)

		// puts file to ecclient
		err = s.ec.Put(ctx, nodes, s.rs, pieceID, sizedReader, expiration)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		p, err = s.makeRemotePointer(nodes, pieceID, sizedReader.Size(), exp, metadata)
		if err != nil {
			return Meta{}, err
		}
	}

	// puts pointer to pointerDB
	err = s.pdb.Put(ctx, path, p)
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

// makeRemotePointer creates a pointer of type remote
func (s *segmentStore) makeRemotePointer(nodes []*opb.Node, pieceID client.PieceID, readerSize int64,
	exp *timestamp.Timestamp, metadata []byte) (pointer *ppb.Pointer, err error) {
	var remotePieces []*ppb.RemotePiece
	for i := range nodes {
		remotePieces = append(remotePieces, &ppb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   nodes[i].Id,
		})
	}

	pointer = &ppb.Pointer{
		Type: ppb.Pointer_REMOTE,
		Remote: &ppb.RemoteSegment{
			Redundancy: &ppb.RedundancyScheme{
				Type:             ppb.RedundancyScheme_RS,
				MinReq:           int32(s.rs.RequiredCount()),
				Total:            int32(s.rs.TotalCount()),
				RepairThreshold:  int32(s.rs.Min),
				SuccessThreshold: int32(s.rs.Opt),
				ErasureShareSize: int32(s.rs.EncodedBlockSize()),
			},
			PieceId:      string(pieceID),
			RemotePieces: remotePieces,
		},
		Size:           readerSize,
		ExpirationDate: exp,
		Metadata:       metadata,
	}
	return pointer, nil
}

// Get retrieves a segment using erasure code, overlay, and pointerdb clients
func (s *segmentStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	if pr.GetType() == ppb.Pointer_REMOTE {
		seg := pr.GetRemote()
		pid := client.PieceID(seg.PieceId)
		nodes, err := s.lookupNodes(ctx, seg)
		if err != nil {
			return nil, Meta{}, Error.Wrap(err)
		}

		es, err := makeErasureScheme(pr.GetRemote().GetRedundancy())
		if err != nil {
			return nil, Meta{}, err
		}

		rr, err = s.ec.Get(ctx, nodes, es, pid, pr.GetSize())
		if err != nil {
			return nil, Meta{}, Error.Wrap(err)
		}
	} else {
		rr = ranger.ByteRangeCloser(pr.InlineSegment)
	}

	return rr, convertMeta(pr), nil
}

func makeErasureScheme(rs *ppb.RedundancyScheme) (eestream.ErasureScheme, error) {
	fc, err := infectious.NewFEC(int(rs.GetMinReq()), int(rs.GetTotal()))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	es := eestream.NewRSScheme(fc, int(rs.GetErasureShareSize()))
	return es, nil
}

// Delete tells piece stores to delete a segment and deletes pointer from pointerdb
func (s *segmentStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path)
	if err != nil {
		return Error.Wrap(err)
	}

	if pr.GetType() == ppb.Pointer_REMOTE {
		seg := pr.GetRemote()
		pid := client.PieceID(seg.PieceId)
		nodes, err := s.lookupNodes(ctx, seg)
		if err != nil {
			return Error.Wrap(err)
		}

		// ecclient sends delete request
		err = s.ec.Delete(ctx, nodes, pid)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	// deletes pointer from pointerdb
	return s.pdb.Delete(ctx, path)
}

// lookupNodes calls Lookup to get node addresses from the overlay
func (s *segmentStore) lookupNodes(ctx context.Context, seg *ppb.RemoteSegment) (
	nodes []*opb.Node, err error) {
	nodes = make([]*opb.Node, len(seg.GetRemotePieces()))
	for i, p := range seg.GetRemotePieces() {
		node, err := s.oc.Lookup(ctx, kademlia.StringToNodeID(p.GetNodeId()))
		if err != nil {
			// TODO(kaloyan): better error handling: failing to lookup a few
			// nodes should not fail the request
			return nil, Error.Wrap(err)
		}
		nodes[i] = node
	}
	return nodes, nil
}

// List retrieves paths to segments and their metadata stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (
	items []ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	pdbItems, more, err := s.pdb.List(ctx, prefix, startAfter, endBefore,
		recursive, limit, metaFlags)
	if err != nil {
		return nil, false, err
	}

	items = make([]ListItem, len(pdbItems))
	for i, itm := range pdbItems {
		items[i] = ListItem{
			Path: itm.Path,
			Meta: convertMeta(itm.Pointer),
		}
	}

	return items, more, nil
}

// convertMeta converts pointer to segment metadata
func convertMeta(pr *ppb.Pointer) Meta {
	return Meta{
		Modified:   convertTime(pr.GetCreationDate()),
		Expiration: convertTime(pr.GetExpirationDate()),
		Size:       pr.GetSize(),
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
