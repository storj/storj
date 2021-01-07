// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
)

// Migrator defines metainfo migrator.
type Migrator struct {
	log           *zap.Logger
	PointerDBStr  string
	MetabaseDBStr string
}

// NewMigrator creates new metainfo migrator.
func NewMigrator(log *zap.Logger, pointerDBStr, metabaseDBStr string) *Migrator {
	return &Migrator{
		log:           log,
		PointerDBStr:  pointerDBStr,
		MetabaseDBStr: metabaseDBStr,
	}
}

// Migrate migrates all entries from pointerdb into metabase.
func (m *Migrator) Migrate(ctx context.Context) (err error) {
	pointerdb, err := metainfo.OpenStore(ctx, m.log.Named("pointerdb"), m.PointerDBStr, "metainfo-migration")
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, pointerdb.Close()) }()

	mb, err := metainfo.OpenMetabase(ctx, m.log.Named("metabase"), m.MetabaseDBStr)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, mb.Close()) }()

	if err := mb.MigrateToLatest(ctx); err != nil {
		return err
	}

	metabaseConn, err := pgx.Connect(ctx, m.MetabaseDBStr)
	if err != nil {
		return fmt.Errorf("unable to connect %q: %w", m.MetabaseDBStr, err)
	}
	defer func() { err = errs.Combine(err, metabaseConn.Close(ctx)) }()

	err = pointerdb.IterateWithoutLookupLimit(ctx, storage.IterateOptions{
		Recurse: true,
		Limit:   500,
	}, func(ctx context.Context, it storage.Iterator) error {
		var item storage.ListItem

		for it.Next(ctx, &item) {
			rawPath := item.Key.String()
			pointer := &pb.Pointer{}

			err := pb.Unmarshal(item.Value, pointer)
			if err != nil {
				return errs.New("unexpected error unmarshalling pointer %s", err)
			}

			location, err := metabase.ParseSegmentKey(metabase.SegmentKey(rawPath))
			if err != nil {
				return err
			}

			// process only last segments
			if location.Position.Index != metabase.LastSegmentIndex {
				continue
			}

			streamID, err := uuid.New()
			if err != nil {
				return err
			}

			streamMeta := &pb.StreamMeta{}
			err = pb.Unmarshal(pointer.Metadata, streamMeta)
			if err != nil {
				return err
			}

			segmentsCount := streamMeta.NumberOfSegments
			if segmentsCount == 0 {
				return errors.New("unsupported case")
			}

			totalEncryptedSize := pointer.SegmentSize
			fixedSegmentSize := pointer.SegmentSize

			// skip inline segment as with metabase implementation we are not storing empty inline segments
			if !(pointer.Type == pb.Pointer_INLINE && len(pointer.InlineSegment) == 0) {
				position := metabase.SegmentPosition{
					Index: uint32(segmentsCount - 1),
				}
				err = insertSegment(ctx, metabaseConn, streamID, position.Encode(), pointer, streamMeta.LastSegmentMeta)
				if err != nil {
					return err
				}
			}

			// TODO use pointerdb.GetAll
			for i := int64(0); i < segmentsCount-1; i++ {
				segmentLocation := location
				segmentLocation.Position.Index = uint32(i) // TODO maybe verify uint32 vs int64

				pointerBytes, err := pointerdb.Get(ctx, storage.Key(segmentLocation.Encode()))
				if err != nil {
					return err
				}

				pointer := &pb.Pointer{}
				err = pb.Unmarshal(pointerBytes, pointer)
				if err != nil {
					return errs.New("unexpected error unmarshalling pointer %s", err)
				}

				totalEncryptedSize += pointer.SegmentSize
				fixedSegmentSize = pointer.SegmentSize

				segmentMeta := &pb.SegmentMeta{}
				err = pb.Unmarshal(pointer.Metadata, segmentMeta)
				if err != nil {
					return errs.New("unexpected error unmarshalling segment meta %s", err)
				}

				err = insertSegment(ctx, metabaseConn, streamID, segmentLocation.Position.Encode(), pointer, segmentMeta)
				if err != nil {
					return err
				}
			}

			encryption, err := encodeEncryption(storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(streamMeta.EncryptionType),
				BlockSize:   streamMeta.EncryptionBlockSize,
			})
			if err != nil {
				return err
			}

			var expireAt *time.Time
			if !pointer.ExpirationDate.IsZero() {
				expireAt = &pointer.ExpirationDate
			}

			_, err = metabaseConn.Exec(ctx, `
				INSERT INTO objects (
					project_id, bucket_name, object_key, version, stream_id,
					created_at, expires_at,
					status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
					total_plain_size, total_encrypted_size, fixed_segment_size,
					encryption
				) VALUES (
					$1, $2, $3, $4, $5,
					$6, $7,
					$8, $9,
					$10, $11, $12,
					$13, $14, $15,
					$16
				)
				`, location.ProjectID, location.BucketName, []byte(location.ObjectKey), 1, streamID,
				pointer.CreationDate, expireAt,
				metabase.Committed, segmentsCount,
				[]byte{}, pointer.Metadata, streamMeta.LastSegmentMeta.EncryptedKey,
				0, totalEncryptedSize, fixedSegmentSize,
				encryption,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func insertSegment(ctx context.Context, metabaseConn *pgx.Conn, streamID uuid.UUID, position uint64, pointer *pb.Pointer, segmentMeta *pb.SegmentMeta) (err error) {
	var rootPieceID storj.PieceID
	var pieces metabase.Pieces
	var redundancy int64
	if pointer.Type == pb.Pointer_REMOTE && pointer.Remote != nil {
		rootPieceID = pointer.Remote.RootPieceId
		redundancy, err = encodeRedundancy(pointer.Remote.Redundancy)
		if err != nil {
			return err
		}

		for _, remotePiece := range pointer.Remote.RemotePieces {
			if remotePiece != nil {
				pieces = append(pieces, metabase.Piece{
					Number:      uint16(remotePiece.PieceNum),
					StorageNode: remotePiece.NodeId,
				})
			}
		}
	}

	_, err = metabaseConn.Exec(ctx, `
		INSERT INTO segments (
			stream_id, position,
			root_piece_id, encrypted_key, encrypted_key_nonce,
			encrypted_size, plain_offset, plain_size,
			redundancy,
			inline_data, remote_pieces
		) VALUES (
			$1, $2,
			$3, $4, $5,
			$6, $7, $8,
			$9,
			$10, $11
		)
		`, streamID, position,
		rootPieceID, segmentMeta.EncryptedKey, segmentMeta.KeyNonce,
		pointer.SegmentSize, 0, 0,
		redundancy,
		pointer.InlineSegment, pieces,
	)
	return err
}

func encodeEncryption(params storj.EncryptionParameters) (int64, error) {
	var bytes [8]byte
	bytes[0] = byte(params.CipherSuite)
	binary.LittleEndian.PutUint32(bytes[1:], uint32(params.BlockSize))
	return int64(binary.LittleEndian.Uint64(bytes[:])), nil
}

func encodeRedundancy(redundancy *pb.RedundancyScheme) (int64, error) {
	params := storj.RedundancyScheme{}
	if redundancy != nil {
		params.Algorithm = storj.RedundancyAlgorithm(redundancy.Type)
		params.ShareSize = redundancy.ErasureShareSize
		params.RequiredShares = int16(redundancy.MinReq)
		params.RepairShares = int16(redundancy.RepairThreshold)
		params.OptimalShares = int16(redundancy.SuccessThreshold)
		params.TotalShares = int16(redundancy.Total)
	}

	var bytes [8]byte
	bytes[0] = byte(params.Algorithm)

	if params.ShareSize >= (1 << 24) {
		return 0, errors.New("redundancy ShareSize is too big to encode")
	}

	bytes[1] = byte(params.ShareSize >> 0)
	bytes[2] = byte(params.ShareSize >> 8)
	bytes[3] = byte(params.ShareSize >> 16)

	bytes[4] = byte(params.RequiredShares)
	bytes[5] = byte(params.RepairShares)
	bytes[6] = byte(params.OptimalShares)
	bytes[7] = byte(params.TotalShares)

	return int64(binary.LittleEndian.Uint64(bytes[:])), nil
}
