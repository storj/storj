// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var _ storj.ReadOnlyStream = (*readonlyStream)(nil)

type readonlyStream struct {
	db *DB

	info      storj.Object
	bucket    string
	encPath   storj.Path
	streamKey *storj.Key // lazySegmentReader derivedKey
}

func (stream *readonlyStream) Info() storj.Object { return stream.info }

func (stream *readonlyStream) SegmentsAt(ctx context.Context, byteOffset int64, limit int64) (infos []storj.Segment, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if stream.info.FixedSegmentSize <= 0 {
		return nil, false, errors.New("not implemented")
	}

	index := byteOffset / stream.info.FixedSegmentSize
	return stream.Segments(ctx, index, limit)
}

func (stream *readonlyStream) segment(ctx context.Context, index int64) (segment storj.Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	segment = storj.Segment{
		Index: index,
	}

	isLastSegment := segment.Index+1 == stream.info.SegmentCount
	if !isLastSegment {
		segmentPath := getSegmentPath(storj.JoinPaths(stream.bucket, stream.encPath), index)
		_, meta, err := stream.db.segments.Get(ctx, segmentPath)
		if err != nil {
			return segment, err
		}

		segmentMeta := pb.SegmentMeta{}
		err = proto.Unmarshal(meta.Data, &segmentMeta)
		if err != nil {
			return segment, err
		}

		segment.Size = stream.info.FixedSegmentSize
		copy(segment.EncryptedKeyNonce[:], segmentMeta.KeyNonce)
		segment.EncryptedKey = segmentMeta.EncryptedKey
	} else {
		segment.Size = stream.info.LastSegment.Size
		segment.EncryptedKeyNonce = stream.info.LastSegment.EncryptedKeyNonce
		segment.EncryptedKey = stream.info.LastSegment.EncryptedKey
	}

	contentKey, err := encryption.DecryptKey(segment.EncryptedKey, stream.Info().EncryptionParameters.CipherSuite, stream.streamKey, &segment.EncryptedKeyNonce)
	if err != nil {
		return segment, err
	}

	nonce := new(storj.Nonce)
	_, err = encryption.Increment(nonce, index+1)
	if err != nil {
		return segment, err
	}

	if isLastSegment {
		index = -1
	}

	pointer, err := stream.db.metainfo.SegmentInfo(ctx, stream.bucket, stream.encPath, index)
	if err != nil {
		return segment, err
	}

	if pointer.GetType() == pb.Pointer_INLINE {
		segment.Inline, err = encryption.Decrypt(pointer.InlineSegment, stream.info.EncryptionParameters.CipherSuite, contentKey, nonce)
	} else {
		segment.PieceID = pointer.Remote.RootPieceId
		segment.Pieces = make([]storj.Piece, 0, len(pointer.Remote.RemotePieces))
		for _, piece := range pointer.Remote.RemotePieces {
			var nodeID storj.NodeID
			copy(nodeID[:], piece.NodeId.Bytes())
			segment.Pieces = append(segment.Pieces, storj.Piece{Number: byte(piece.PieceNum), Location: nodeID})
		}
	}

	return segment, nil
}

func (stream *readonlyStream) Segments(ctx context.Context, index int64, limit int64) (infos []storj.Segment, more bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if index < 0 {
		return nil, false, errors.New("invalid argument")
	}
	if limit <= 0 {
		limit = defaultSegmentLimit
	}
	if index >= stream.info.SegmentCount {
		return nil, false, nil
	}

	infos = make([]storj.Segment, 0, limit)
	for ; index < stream.info.SegmentCount && limit > 0; index++ {
		limit--
		segment, err := stream.segment(ctx, index)
		if err != nil {
			return nil, false, err
		}
		infos = append(infos, segment)
	}

	more = index < stream.info.SegmentCount
	return infos, more, nil
}

type mutableStream struct {
	db   *DB
	info storj.Object
}

func (stream *mutableStream) Info() storj.Object { return stream.info }

func (stream *mutableStream) AddSegments(ctx context.Context, segments ...storj.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errors.New("not implemented")
}

func (stream *mutableStream) UpdateSegments(ctx context.Context, segments ...storj.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errors.New("not implemented")
}
