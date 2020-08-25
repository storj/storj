package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"storj.io/common/pb"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
)

type Migrator struct {
	PointerDB metainfo.PointerDB
	Metabase  *Metabase

	ProjectID  uuid.UUID
	BucketID   uuid.UUID
	BucketName []byte

	Batch     *pgx.Batch
	BatchSize int
}

func NewMigrator(db metainfo.PointerDB, metabase *Metabase, projectID uuid.UUID, bucketID uuid.UUID, bucketName []byte) *Migrator {
	return &Migrator{
		PointerDB: db,
		Metabase:  metabase,

		ProjectID:  projectID,
		BucketName: bucketName,

		Batch:     &pgx.Batch{},
		BatchSize: 500,
	}
}

func (m *Migrator) MigrateBucket(ctx context.Context) error {
	path, err := metainfo.CreatePath(ctx, m.ProjectID, -1, m.BucketName, nil)
	if err != nil {
		return err
	}

	more := true
	lastKey := storage.Key{}
	for more {
		more, err = storage.ListV2Iterate(ctx, m.PointerDB, storage.ListOptions{
			Prefix:       storage.Key(path),
			StartAfter:   lastKey,
			Recursive:    true,
			Limit:        int(0),
			IncludeValue: true,
		}, func(ctx context.Context, item *storage.ListItem) error {
			pointer := &pb.Pointer{}
			err = pb.Unmarshal(item.Value, pointer)
			if err != nil {
				return err
			}

			encodedPath := item.Key.String()
			if encodedPath[0] == '/' {
				encodedPath = encodedPath[1:]
			}

			err = m.insertObject(ctx, []byte(encodedPath), pointer)
			if err != nil {
				return err
			}

			lastKey = item.Key
			return nil
		})
		if err != nil {
			return err
		}
	}

	if m.Batch.Len() > 0 {
		br := m.Metabase.conn.SendBatch(ctx, m.Batch)
		err := br.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) insertObject(ctx context.Context, encryptedPath []byte, pointer *pb.Pointer) error {
	streamMeta := &pb.StreamMeta{}
	err := pb.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return err
	}

	segmentsCount := streamMeta.NumberOfSegments
	if segmentsCount == 0 {
		return errors.New("unsupported case")
	}

	streamID, err := NewUUID()
	if err != nil {
		return err
	}

	err = m.execute(ctx, `
		INSERT INTO objects (
			project_id, bucket_id, encrypted_path, version, stream_id,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata_nonce
		) VALUES (
			$1, $2, $3, $4, $5, 
			$6, $7,
			$8, $9,
			$10
			--encrypted_metadata_nonce
		)
	`, m.ProjectID, m.BucketID, encryptedPath, -1, streamID,
		pointer.CreationDate, pointer.ExpirationDate,
		"committed", segmentsCount,
		pointer.Metadata,
	)
	if err != nil {
		return err
	}

	err = m.insertSegment(ctx, streamID, segmentsCount-1, pointer)
	if err != nil {
		return err
	}

	for i := int64(0); i < segmentsCount-1; i++ {
		path, err := metainfo.CreatePath(ctx, m.ProjectID, i, m.BucketName, encryptedPath)
		if err != nil {
			return err
		}

		value, err := m.PointerDB.Get(ctx, storage.Key(path))
		if err != nil {
			// TODO drop whole object if one segment is missing (zombie segment)
			return err
		}

		segmentPointer := &pb.Pointer{}
		err = pb.Unmarshal(value, segmentPointer)
		if err != nil {
			return err
		}

		err = m.insertSegment(ctx, streamID, i, segmentPointer)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) insertSegment(ctx context.Context, streamID UUID, segmentIndex int64, pointer *pb.Pointer) error {
	segmentPosition := SegmentPosition{
		Part:    0,
		Segment: uint32(segmentIndex),
	}

	rootPieceID := []byte{}
	if pointer.Remote != nil {
		rootPieceID = pointer.Remote.RootPieceId.Bytes()
	}

	streamMeta := &pb.StreamMeta{}
	err := pb.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return err
	}

	encryptedKey := []byte{}
	encryptedKeyNonce := []byte{}
	if streamMeta.LastSegmentMeta != nil {
		encryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
		encryptedKeyNonce = streamMeta.LastSegmentMeta.KeyNonce
	}

	err = m.execute(ctx, `
	INSERT INTO segments (
		stream_id, segment_position, root_piece_id,
		encrypted_key, encrypted_key_nonce,
		data_size, inline_data,
		node_aliases
	) VALUES (
		$1, $2, $3,
		$4, $5,
		$6, $7,
		$8
	)
	`, streamID, segmentPosition.Encode(), rootPieceID,
		encryptedKey, encryptedKeyNonce,
		int32(pointer.SegmentSize), pointer.InlineSegment,
		NodeAliases{1}.Encode(),
	)
	return err
}

func (m *Migrator) execute(ctx context.Context, sql string, arguments ...interface{}) error {
	m.Batch.Queue(sql, arguments...)

	if m.Batch.Len() >= m.BatchSize {
		br := m.Metabase.conn.SendBatch(ctx, m.Batch)
		err := br.Close()
		if err != nil {
			return err
		}

		m.Batch = &pgx.Batch{}
	}

	return nil
}
