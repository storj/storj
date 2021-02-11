// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"os"
	"runtime/pprof"
	"sort"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/storj/cmd/metainfo-migration/fastpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
)

// Config initial settings for verifier.
type Config struct {
	SamplePercent float64
}

// Verifier defines metainfo migration verifier.
type Verifier struct {
	log           *zap.Logger
	pointerDBStr  string
	metabaseDBStr string
	config        Config
}

// NewVerifier creates new metainfo migration verifier.
func NewVerifier(log *zap.Logger, pointerDBStr, metabaseDBStr string, config Config) *Verifier {
	if config.SamplePercent == 0 {
		config.SamplePercent = defaultSamplePercent
	}
	return &Verifier{
		log:           log,
		pointerDBStr:  pointerDBStr,
		metabaseDBStr: metabaseDBStr,
		config:        config,
	}
}

// VerifyPointers verifies a sample of all migrated pointers.
func (v *Verifier) VerifyPointers(ctx context.Context) (err error) {
	v.log.Debug("Databases", zap.String("PointerDB", v.pointerDBStr), zap.String("MetabaseDB", v.metabaseDBStr))

	pointerDBConn, err := pgx.Connect(ctx, v.pointerDBStr)
	if err != nil {
		return errs.New("unable to connect %q: %w", v.pointerDBStr, err)
	}
	defer func() { err = errs.Combine(err, pointerDBConn.Close(ctx)) }()

	mb, err := metainfo.OpenMetabase(ctx, v.log.Named("metabase"), v.metabaseDBStr)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, mb.Close()) }()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	pointer := &fastpb.Pointer{}
	streamMeta := &fastpb.StreamMeta{}
	segmentMeta := &fastpb.SegmentMeta{}
	var fullpath, metadata []byte
	var allSegments int64
	var lastSegment bool

	start := time.Now()

	v.log.Info("Start", zap.Time("time", start),
		zap.Float64("samplePercent", v.config.SamplePercent),
	)

	rows, err := pointerDBConn.Query(ctx, `SELECT fullpath, metadata FROM pathdata WHERE RANDOM() < $1`, v.config.SamplePercent/100)
	if err != nil {
		return err
	}
	defer func() { rows.Close() }()

	lastCheck := time.Now()
	for rows.Next() {
		err = rows.Scan(&fullpath, &metadata)
		if err != nil {
			return err
		}

		segmentLocation, err := metabase.ParseSegmentKey(metabase.SegmentKey(fullpath))
		if err != nil {
			return errs.New("%v; pointer: %s", err, fullpath)
		}

		err = proto.Unmarshal(metadata, pointer)
		if err != nil {
			return wrap(err, segmentLocation)
		}

		lastSegment = segmentLocation.Position.Index == metabase.LastSegmentIndex
		if lastSegment {
			err = proto.Unmarshal(pointer.Metadata, streamMeta)
			if err != nil {
				return wrap(err, segmentLocation)
			}
			// calculate the metabase segment position, so we can query the segment from the metabase
			segmentLocation.Position.Index = uint32(streamMeta.NumberOfSegments) - 1
		}

		segment, err := mb.GetSegmentByLocation(ctx, metabase.GetSegmentByLocation{SegmentLocation: segmentLocation})
		if err != nil {
			return wrap(err, segmentLocation)
		}

		if pointer.Type == fastpb.Pointer_DataType(pb.Pointer_REMOTE) {
			if len(segment.InlineData) > 0 {
				return wrap(errs.New("unexpected inline data for remote segment"), segmentLocation)
			}
			if pointer.Remote.RootPieceId != segment.RootPieceID {
				return wrap(errs.New("root piece id does not match: want %s, got %s", pointer.Remote.RootPieceId, segment.RootPieceID), segmentLocation)
			}
			if pointer.Remote.Redundancy.Type != fastpb.RedundancyScheme_SchemeType(segment.Redundancy.Algorithm) {
				return wrap(errs.New("redundancy scheme type does not match: want %d, got %d", pointer.Remote.Redundancy.Type, segment.Redundancy.Algorithm), segmentLocation)
			}
			if pointer.Remote.Redundancy.MinReq != int32(segment.Redundancy.RequiredShares) {
				return wrap(errs.New("redundancy scheme required shares does not match: want %d, got %d", pointer.Remote.Redundancy.MinReq, segment.Redundancy.RequiredShares), segmentLocation)
			}
			if pointer.Remote.Redundancy.RepairThreshold != int32(segment.Redundancy.RepairShares) {
				return wrap(errs.New("redundancy scheme repair shares does not match: want %d, got %d", pointer.Remote.Redundancy.RepairThreshold, segment.Redundancy.RepairShares), segmentLocation)
			}
			if pointer.Remote.Redundancy.SuccessThreshold != int32(segment.Redundancy.OptimalShares) {
				return wrap(errs.New("redundancy scheme optimal shares does not match: want %d, got %d", pointer.Remote.Redundancy.SuccessThreshold, segment.Redundancy.OptimalShares), segmentLocation)
			}
			if pointer.Remote.Redundancy.Total != int32(segment.Redundancy.TotalShares) {
				return wrap(errs.New("redundancy scheme total shares does not match: want %d, got %d", pointer.Remote.Redundancy.Total, segment.Redundancy.TotalShares), segmentLocation)
			}
			if pointer.Remote.Redundancy.ErasureShareSize != segment.Redundancy.ShareSize {
				return wrap(errs.New("redundancy scheme erasure share size does not match: want %d, got %d", pointer.Remote.Redundancy.ErasureShareSize, segment.Redundancy.ShareSize), segmentLocation)
			}
			if len(pointer.Remote.RemotePieces) != segment.Pieces.Len() {
				return wrap(errs.New("number of remote pieces does not match: want %d, got %d", len(pointer.Remote.RemotePieces), segment.Pieces.Len()), segmentLocation)
			}
			sort.Slice(pointer.Remote.RemotePieces, func(i, k int) bool {
				return pointer.Remote.RemotePieces[i].PieceNum < pointer.Remote.RemotePieces[k].PieceNum
			})
			sort.Slice(segment.Pieces, func(i, k int) bool {
				return segment.Pieces[i].Number < segment.Pieces[k].Number
			})
			for i, piece := range pointer.Remote.RemotePieces {
				if piece.PieceNum != int32(segment.Pieces[i].Number) {
					return wrap(errs.New("piece number does not match for remote piece %d: want %d, got %d", i, piece.PieceNum, segment.Pieces[i].Number), segmentLocation)
				}
				if piece.NodeId != segment.Pieces[i].StorageNode {
					return wrap(errs.New("storage node id does not match for remote piece %d: want %s, got %s", i, piece.NodeId, segment.Pieces[i].StorageNode), segmentLocation)
				}
			}
		} else {
			if !bytes.Equal(pointer.InlineSegment, segment.InlineData) {
				return wrap(errs.New("inline data does not match: want %x, got %x", pointer.InlineSegment, segment.InlineData), segmentLocation)
			}
			if !segment.RootPieceID.IsZero() {
				return wrap(errs.New("unexpected root piece id for inline segment"), segmentLocation)
			}
			if !segment.Redundancy.IsZero() {
				return wrap(errs.New("unexpected redundancy scheme for inline segment"), segmentLocation)
			}
			if segment.Pieces.Len() > 0 {
				return wrap(errs.New("unexpected remote pieces for inline segment"), segmentLocation)
			}
		}

		if segment.StreamID.IsZero() {
			return wrap(errs.New("missing stream id in segment"), segmentLocation)
		}
		if pointer.SegmentSize != int64(segment.EncryptedSize) {
			return wrap(errs.New("segment size does not match: want %d, got %d", pointer.SegmentSize, segment.EncryptedSize), segmentLocation)
		}
		if segment.PlainOffset != 0 {
			return wrap(errs.New("unexpected plain offset: %d", segment.PlainOffset), segmentLocation)
		}
		if segment.PlainSize != 0 {
			return wrap(errs.New("unexpected plain size: %d", segment.PlainSize), segmentLocation)
		}

		if lastSegment {
			object, err := mb.GetObjectLatestVersion(ctx, metabase.GetObjectLatestVersion{ObjectLocation: segmentLocation.Object()})
			if err != nil {
				return wrap(err, segmentLocation)
			}
			if object.StreamID.IsZero() {
				return wrap(errs.New("missing stream id in object"), segmentLocation)
			}
			if object.StreamID != segment.StreamID {
				return wrap(errs.New("stream id does no match: object %s, segment %s", object.StreamID, segment.StreamID), segmentLocation)
			}
			if object.Version != 1 {
				return wrap(errs.New("unexpected version: want %d, got %d", 1, object.Version), segmentLocation)
			}
			if object.Status != metabase.Committed {
				return wrap(errs.New("unexpected status: want %d, got %d", metabase.Committed, object.Status), segmentLocation)
			}
			if !withinDuration(pointer.CreationDate, object.CreatedAt, 1*time.Microsecond) {
				return wrap(errs.New("creation date does not match: want %s, got %s", pointer.CreationDate, object.CreatedAt), segmentLocation)
			}
			if object.ExpiresAt == nil {
				if !pointer.ExpirationDate.IsZero() {
					return wrap(errs.New("missing expiration date"), segmentLocation)
				}
			} else if !withinDuration(pointer.ExpirationDate, *object.ExpiresAt, 1*time.Microsecond) {
				return wrap(errs.New("expiration date does not match: want %s, got %s", pointer.ExpirationDate, object.ExpiresAt), segmentLocation)
			}
			if int32(streamMeta.NumberOfSegments) != object.SegmentCount {
				return wrap(errs.New("number of segments does not match: want %d, got %d", streamMeta.NumberOfSegments, object.SegmentCount), segmentLocation)
			}
			if object.FixedSegmentSize != 0 {
				return wrap(errs.New("unexpected fixed segment size: %d", object.FixedSegmentSize), segmentLocation)
			}
			if object.SegmentCount == 1 {
				if pointer.SegmentSize != object.TotalEncryptedSize {
					return wrap(errs.New("total encrypted size does not match: want %d, got %d", pointer.SegmentSize, object.TotalEncryptedSize), segmentLocation)
				}
			} else {
				if pointer.SegmentSize >= object.TotalEncryptedSize {
					return wrap(errs.New("total encrypted size does not match: want >%d, got %d", pointer.SegmentSize, object.TotalEncryptedSize), segmentLocation)
				}
			}
			if object.TotalPlainSize != 0 {
				return wrap(errs.New("unexpected total plain size: %d", object.TotalPlainSize), segmentLocation)
			}
			if streamMeta.EncryptionType != int32(object.Encryption.CipherSuite) {
				return wrap(errs.New("encryption type does not match: want %d, got %d", streamMeta.EncryptionType, object.Encryption.CipherSuite), segmentLocation)
			}
			if streamMeta.EncryptionBlockSize != object.Encryption.BlockSize {
				return wrap(errs.New("encryption block size does not match: want %d, got %d", streamMeta.EncryptionBlockSize, object.Encryption.BlockSize), segmentLocation)
			}
			if !bytes.Equal(streamMeta.LastSegmentMeta.EncryptedKey, object.EncryptedMetadataEncryptedKey) {
				return wrap(errs.New("encrypted metadata encrypted key does not match: want %x, got %x", streamMeta.LastSegmentMeta.EncryptedKey, object.EncryptedMetadataEncryptedKey), segmentLocation)
			}
			if !bytes.Equal(streamMeta.LastSegmentMeta.KeyNonce, object.EncryptedMetadataNonce) {
				return wrap(errs.New("encrypted metadata key nonce does not match: want %x, got %x", streamMeta.LastSegmentMeta.KeyNonce, object.EncryptedMetadataNonce), segmentLocation)
			}
			if !bytes.Equal(pointer.Metadata, object.EncryptedMetadata) {
				return wrap(errs.New("encrypted metadata does not match: want %x, got %x", pointer.Metadata, object.EncryptedMetadata), segmentLocation)
			}
			if object.ZombieDeletionDeadline != nil {
				return wrap(errs.New("unexpected zombie deletion deadline: %s", object.ZombieDeletionDeadline), segmentLocation)
			}
		} else {
			err = pb.Unmarshal(pointer.Metadata, segmentMeta)
			if err != nil {
				return wrap(err, segmentLocation)
			}
			if !bytes.Equal(segmentMeta.EncryptedKey, segment.EncryptedKey) {
				return wrap(errs.New("segment metadata encrypted key does not match: want %x, got %x", segmentMeta.EncryptedKey, segment.EncryptedKey), segmentLocation)
			}
			if !bytes.Equal(segmentMeta.KeyNonce, segment.EncryptedKeyNonce) {
				return wrap(errs.New("segment metadata key nonce does not match: want %x, got %x", segmentMeta.KeyNonce, segment.EncryptedKeyNonce), segmentLocation)
			}
		}

		if allSegments != 0 && allSegments%100 == 0 {
			v.log.Info("Processed segments", zap.Int64("segments", allSegments), zap.Duration("took", time.Since(lastCheck)))
			lastCheck = time.Now()
		}

		allSegments++
	}

	v.log.Info("Finished", zap.Int64("segments", allSegments), zap.Duration("Total", time.Since(start)))

	return rows.Err()
}

func withinDuration(expected, actual time.Time, delta time.Duration) bool {
	dt := expected.Sub(actual)
	return -delta < dt && dt < delta
}

func wrap(err error, segment metabase.SegmentLocation) error {
	return errs.New("%v; project: %x, bucket: %s, object: %x, index: %d",
		err, segment.ProjectID, segment.BucketName, segment.ObjectKey, segment.Position.Index)
}
