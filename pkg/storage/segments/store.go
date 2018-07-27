// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segment

import (
	"context"
	"io"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/ec"
	opb "storj.io/storj/protos/overlay"
	ppb "storj.io/storj/protos/pointerdb"
)

var (
	mon = monkit.Package()
)

// Meta will contain encryption and stream information
type Meta struct {
	Data []byte
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
		recursive bool, limit int, metaFlags uint64) (items []storage.ListItem,
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

// PeekThresholdReader allows a check to see if the size of a given reader
// exceeds the maximum inline segment size or not.
type PeekThresholdReader struct {
	r              io.Reader
	n              int // number of bytes read into thresholdBuf
	thresholdBuf   []byte
	totalReadBytes int
}

// NewPeekThresholdReader creates a new instance of PeekThresholdReader
func NewPeekThresholdReader(r io.Reader) (pt *PeekThresholdReader) {
	return &PeekThresholdReader{r: r, n: 0, thresholdBuf: nil, totalReadBytes: 0}
}

// Read initially reads bytes from the internal buffer, then continues
// reading from the wrapped data reader. The number of bytes read `n`
// is returned.
func (pt *PeekThresholdReader) Read(p []byte) (n int, err error) {

	// Case 1: if the total number of bytes read is greater than the number
	// of bytes read into the thresholdBuf, then Read is called on the given
	// byte slice.
	if pt.totalReadBytes > pt.n {
		return pt.r.Read(p)
	}

	thresholdBytesRemaining := pt.n - pt.totalReadBytes

	// Case 2: if the length of the given byte slice p is less than or equal
	// to the number of threshold bytes remaining to be read, then the slice
	// of the threshold buffer from the end of totalReadBytes to the length
	// of p is copied to p.
	if len(p) <= thresholdBytesRemaining {
		tmp := pt.thresholdBuf[pt.totalReadBytes : pt.totalReadBytes+len(p)]
		copy(p, tmp)
		pt.totalReadBytes += len(p)
		return len(p), nil
	}

	// Case 3: The buffer tail slice is created then read.
	// A slice of read bytes is copied to the given byte slice p.
	tmp := pt.thresholdBuf[pt.totalReadBytes : pt.totalReadBytes+thresholdBytesRemaining]
	bufTail := make([]byte, len(p)-thresholdBytesRemaining)
	numTailBytes, err := pt.r.Read(bufTail)
	if err != nil {
		return 0, err
	}
	tmp = append(tmp, bufTail...)
	copy(p, tmp)
	n = thresholdBytesRemaining + numTailBytes
	pt.totalReadBytes += n
	return n, nil
}

// checkSize returns a bool to determine whether a reader's size
// is inline-sized or not.
func (pt *PeekThresholdReader) isInline(thresholdSize int) (inline bool, err error) {
	err = pt.makeThresholdBuffer(thresholdSize)
	if err != nil {
		return false, err
	}
	if pt.n < thresholdSize {
		return true, nil
	}
	return false, nil
}

func (pt *PeekThresholdReader) makeThresholdBuffer(thresholdSize int) (err error) {
	buf := make([]byte, thresholdSize)
	n, err := pt.r.Read(buf)
	if err != nil {
		return err
	}
	pt.n = n
	pt.thresholdBuf = buf
	return nil
}

// Meta retrieves the metadata of the segment
func (s *segmentStore) Meta(ctx context.Context, path paths.Path) (meta Meta,
	err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return Meta{}, Error.Wrap(err)
	}

	return Meta{Data: pr.GetMetadata()}, nil
}

// Put uploads a segment to an erasure code client
func (s *segmentStore) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	var p *ppb.Pointer

	peekReader := NewPeekThresholdReader(data)
	remoteSized, err := peekReader.isInline(s.thresholdSize)
	if err != nil {
		return Meta{}, err
	}
	if !remoteSized {
		p = &ppb.Pointer{
			Type:     ppb.Pointer_INLINE,
			Metadata: metadata,
		}
	} else {
		// uses overlay client to request a list of nodes
		nodes, err := s.oc.Choose(ctx, s.rs.TotalCount(), 0)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		pieceID := client.NewPieceID()

		// puts file to ecclient
		err = s.ec.Put(ctx, nodes, s.rs, pieceID, data, expiration)
		if err != nil {
			return Meta{}, Error.Wrap(err)
		}
		p = s.makeRemotePointer(nodes, pieceID, metadata)
	}

	// puts pointer to pointerDB
	err = s.pdb.Put(ctx, path, p, nil)
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
func (s *segmentStore) makeRemotePointer(nodes []*opb.Node, pieceID client.PieceID,
	metadata []byte) (pointer *ppb.Pointer) {
	var remotePieces []*ppb.RemotePiece
	for i := range nodes {
		remotePieces = append(remotePieces, &ppb.RemotePiece{
			PieceNum: int64(i),
			NodeId:   nodes[i].Id,
		})
	}
	pointer = &ppb.Pointer{
		Type: ppb.Pointer_REMOTE,
		Remote: &ppb.RemoteSegment{
			Redundancy: &ppb.RedundancyScheme{
				Type:             ppb.RedundancyScheme_RS,
				MinReq:           int64(s.rs.RequiredCount()),
				Total:            int64(s.rs.TotalCount()),
				RepairThreshold:  int64(s.rs.Min),
				SuccessThreshold: int64(s.rs.Opt),
			},
			PieceId:      string(pieceID),
			RemotePieces: remotePieces,
		},
		Metadata: metadata,
	}
	return pointer
}

// Get retrieves a segment using erasure code, overlay, and pointerdb clients
func (s *segmentStore) Get(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta Meta, err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	if pr.GetType() != ppb.Pointer_REMOTE {
		return nil, Meta{}, Error.New("TODO: only getting remote pointers supported")
	}

	seg := pr.GetRemote()
	pid := client.PieceID(seg.PieceId)
	nodes, err := s.lookupNodes(ctx, seg)
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	rr, err = s.ec.Get(ctx, nodes, s.rs, pid, pr.GetSize())
	if err != nil {
		return nil, Meta{}, Error.Wrap(err)
	}

	return rr, Meta{Data: pr.GetMetadata()}, nil
}

// Delete tells piece stores to delete a segment and deletes pointer from pointerdb
func (s *segmentStore) Delete(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	pr, err := s.pdb.Get(ctx, path, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	if pr.GetType() != ppb.Pointer_REMOTE {
		return Error.New("TODO: only getting remote pointers supported")
	}

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

	// deletes pointer from pointerdb
	return s.pdb.Delete(ctx, path, nil)
}

// lookupNodes calls Lookup to get node addresses from the overlay
func (s *segmentStore) lookupNodes(ctx context.Context, seg *ppb.RemoteSegment) (
	nodes []*opb.Node, err error) {
	nodes = make([]*opb.Node, len(seg.GetRemotePieces()))
	for i, p := range seg.GetRemotePieces() {
		node, err := s.oc.Lookup(ctx, kademlia.StringToNodeID(p.GetNodeId()))
		if err != nil {
			// TODO better error handling: failing to lookup a few nodes should
			// not fail the request
			return nil, Error.Wrap(err)
		}
		nodes[i] = node
	}
	return nodes, nil
}

// List retrieves paths to segments and their metadata stored in the pointerdb
func (s *segmentStore) List(ctx context.Context, prefix, startAfter,
	endBefore paths.Path, recursive bool, limit int, metaFlags uint64) (
	items []storage.ListItem, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	res, more, err := s.pdb.List(ctx, startAfter, int64(limit), nil)
	if err != nil {
		return nil, false, Error.Wrap(err)
	}

	items = make([]storage.ListItem, len(res))

	for i, path := range res {
		items[i].Path = paths.New(string(path))
		// TODO items[i].Meta =
	}

	return items, more, nil
}
