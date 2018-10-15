// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/ranger"
	ecclient "storj.io/storj/pkg/storage/ec"
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
	Path     paths.Path
	Meta     Meta
	IsPrefix bool
}

// Store for segments
type Store interface {
	Meta(ctx context.Context, path paths.Path) (meta Meta, err error)
	Get(ctx context.Context, path paths.Path) (rr ranger.Ranger, meta Meta, err error)
	Repair(ctx context.Context, path paths.Path, lostPieces []int) (err error)
	Put(ctx context.Context, data io.Reader, expiration time.Time, segmentInfo func() (paths.Path, []byte, error)) (meta Meta, err error)
	Delete(ctx context.Context, path paths.Path) (err error)
	List(ctx context.Context, prefix, startAfter, endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (items []ListItem, more bool, err error)
}

type segmentStore struct {
	oc            overlay.Client
	ec            ecclient.Client
	pdb           pdbclient.Client
	rs            eestream.RedundancyStrategy
	thresholdSize int
}

// NewSegmentStore creates a new instance of segmentStore
func NewSegmentStore(oc overlay.Client, ec ecclient.Client,
	pdb pdbclient.Client, rs eestream.RedundancyStrategy, t int) Store {
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
func (s *segmentStore) Put(ctx context.Context, data io.Reader, expiration time.Time, segmentInfo func() (paths.Path, []byte, error)) (meta Meta, err error) {
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

	var path paths.Path
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
			Size:           int64(len(peekReader.thresholdBuf)),
			ExpirationDate: exp,
			Metadata:       metadata,
		}
	} else {
		// uses overlay client to request a list of nodes
		nodes, err := s.oc.Choose(ctx, overlay.Options{Amount: s.rs.TotalCount(), Space: 0, Excluded: nil})
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		pieceID := client.NewPieceID()
		sizedReader := SizeReader(peekReader)

		// puts file to ecclient
		successfulNodes, err := s.ec.Put(ctx, nodes, s.rs, pieceID, sizedReader, expiration)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}

		p, metadata, err := segmentInfo()
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		path = p

		pointer, err = s.makeRemotePointer(successfulNodes, pieceID, sizedReader.Size(), exp, metadata)
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

// makeRemotePointer creates a pointer of type remote
func (s *segmentStore) makeRemotePointer(nodes []*pb.Node, pieceID client.PieceID, readerSize int64,
	exp *timestamp.Timestamp, metadata []byte) (pointer *pb.Pointer, err error) {
	var remotePieces []*pb.RemotePiece
	for i := range nodes {
		if nodes[i] == nil {
			continue
		}
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
				MinReq:           int32(s.rs.RequiredCount()),
				Total:            int32(s.rs.TotalCount()),
				RepairThreshold:  int32(s.rs.RepairThreshold()),
				SuccessThreshold: int32(s.rs.OptimalThreshold()),
				ErasureShareSize: int32(s.rs.ErasureShareSize()),
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
	rr ranger.Ranger, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	//c := make(chan bool, 1)
	log.Println("KISHORE --> Testing the repair START ******")
	lostPieces := []int{1, 2}
	s.Repair(ctx, path, lostPieces)

	log.Println("KISHORE --> Testing the repair END ******")
	//<-c
	pr, err := s.pdb.Get(ctx, path)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	if pr.GetType() == pb.Pointer_REMOTE {
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
		rr = ranger.ByteRanger(pr.InlineSegment)
	}

	return rr, convertMeta(pr), nil
}

func makeErasureScheme(rs *pb.RedundancyScheme) (eestream.ErasureScheme, error) {
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

	if pr.GetType() == pb.Pointer_REMOTE {
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

// Repair retrieves an at-risk segment and repairs and stores lost pieces on new nodes
func (s *segmentStore) Repair(ctx context.Context, path paths.Path, lostPieces []int) (err error) {
	defer mon.Task()(&ctx)(&err)

	//Read the segment's pointer's info from the PointerDB
	pr, err := s.pdb.Get(ctx, path)
	if err != nil {
		return Error.Wrap(err)
	}

	if pr.GetType() == pb.Pointer_REMOTE {
		seg := pr.GetRemote()
		pid := client.PieceID(seg.PieceId)
		log.Println("KISHORE --> segment pid ", pid)

		// Get the list of remote pieces from the pointer
		originalNodes, err := s.lookupNodes(ctx, seg)
		if err != nil {
			return Error.Wrap(err)
		}
		log.Println("KISHORE -->  length of originalNodes =", len(originalNodes))
		nilNodes := 0
		for i := range originalNodes {
			log.Println("KISHORE -->  originalNodes [", i, "] = ", originalNodes[i])
			if originalNodes[i] == nil {
				nilNodes = nilNodes + 1
				continue
			}
		}
		log.Println("KISHORE -->  total nil nodes =", nilNodes)

		var excludeNodeIDs []dht.NodeID
		for _, v := range originalNodes {
			if v != nil {
				excludeNodeIDs = append(excludeNodeIDs, node.IDFromString(v.Id))
			}
		}
		log.Println("KISHORE -->  list of excluded nodeIDs =", excludeNodeIDs)

		//Request Overlay for n-h new storage nodes
		newNodes, err := s.getNewUniqueNodes(ctx, excludeNodeIDs, (len(lostPieces) + nilNodes), 0)
		log.Println("KISHORE --> list of unique nodes", newNodes)

		//remove all lost pieces from the list to have only healthy pieces
		for j := range originalNodes {
			for i := range lostPieces {
				if j == lostPieces[i] {
					originalNodes[j] = nil
					log.Println("KISHORE --> replacing original node", j, "with nil ", originalNodes[j])
				}
			}
		}
		log.Println("KISHORE --> list of nodes after", originalNodes)

		log.Println("KISHORE -->  start of repair work")
		es, err := makeErasureScheme(pr.GetRemote().GetRedundancy())
		if err != nil {
			return err
		}

		// download the segment using the nodes without bad nodes
		rr, err := s.ec.Get(ctx, originalNodes, es, pid, pr.GetSize())
		if err != nil {
			return Error.Wrap(err)
		}

		log.Println("KISHORE -->  downloaded the segment that needs to be repaired")
		// get io.Reader from ranger
		r, err := rr.Range(ctx, 0, rr.Size())
		if err != nil {
			return err
		}
		log.Println("KISHORE -->  downloaded the of the segment using healthy nodes list, size =", rr.Size())
		log.Println("KISHORE -->  print ranger", r)

		/* to really make the piecenodes unique test code */
		log.Println("KISHORE -->  making the piecestore nodes unique start")
		// ecclient sends delete request
		err = s.ec.Delete(ctx, newNodes, pid)
		if err != nil {
			return Error.Wrap(err)
		}
		log.Println("KISHORE -->  making the piecestore nodes unique done")

		// puts file to ecclient
		exp := pr.GetExpirationDate()

		successfulNodes, err := s.ec.Repair(ctx, originalNodes, newNodes, s.rs, pid, r, time.Time{})
		if err != nil {
			log.Println("KISHORE --> error in putting the pieces")
			return Error.Wrap(err)
		}
		log.Println("KISHORE --> uploading new pieceIDs replaceing lost pieces", successfulNodes)

		metadata := pr.GetMetadata()

		//
		pointer, err := s.makeRemotePointer(successfulNodes, pid, rr.Size(), exp, metadata)
		if err != nil {
			return err
		}
		// puts pointer to pointerDB
		err = s.pdb.Put(ctx, path, pointer)
		return err
	}

	zap.S().Error("Shouldn't be here.....: ", err)
	return errs.New("Cannot repair inline segment")
}

// getNewUniqueNodes gets a list of new nodes different from the passed nodes list
func (s *segmentStore) getNewUniqueNodes(ctx context.Context, nodes []dht.NodeID, numOfNodes int, space int64) ([]*pb.Node, error) {
	op := overlay.Options{Amount: numOfNodes, Space: space, Excluded: nodes}
	newNodes, err := s.oc.Choose(ctx, op)
	if err != nil {
		return nil, err
	}

	// if nodes == nil {
	// 	return newNodes, err
	// }
	// log.Println("KISHORE --> list of newNodes ", newNodes)

	// uniqueNodes := len(newNodes)
	// for uniqueNodes > 0 {
	// 	for j := range newNodes {
	// 		for i := range nodes {

	// 			if newNodes[j] == nodes[i] {
	// 				uniqueNodes = uniqueNodes + 1
	// 			}
	// 		}
	// 		uniqueNodes = (uniqueNodes - 1)
	// 		log.Println("KISHORE --> uniqueNodes = ", uniqueNodes)
	// 	}
	// 	if uniqueNodes > 0 {
	// 		// request a new set of nodes
	// 		newNodes, err = s.oc.Choose(ctx, numOfNodes, space)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// 	log.Println("KISHORE --> got unique nodes")
	// }
	return newNodes, err
}

// lookupNodes calls Lookup to get node addresses from the overlay
func (s *segmentStore) lookupNodes(ctx context.Context, seg *pb.RemoteSegment) (nodes []*pb.Node, err error) {
	// Get list of all nodes IDs storing a piece from the segment
	var nodeIds []dht.NodeID
	for _, p := range seg.RemotePieces {
		nodeIds = append(nodeIds, node.IDFromString(p.GetNodeId()))
	}
	// Lookup the node info from node IDs
	n, err := s.oc.BulkLookup(ctx, nodeIds)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	// Create an indexed list of nodes based on the piece number.
	// Missing pieces are represented by a nil node.
	nodes = make([]*pb.Node, seg.GetRedundancy().GetTotal())
	for i, p := range seg.GetRemotePieces() {
		nodes[p.PieceNum] = n[i]
	}
	return nodes, nil
}

// List retrieves paths to segments and their metadata stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint32) (
	items []ListItem, more bool, err error) {
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

// convertMeta converts pointer to segment metadata
func convertMeta(pr *pb.Pointer) Meta {
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
