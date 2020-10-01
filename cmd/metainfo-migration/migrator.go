package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"storj.io/common/pb"
	"storj.io/common/uuid"
	"storj.io/storj/cmd/metainfo-migration/metabase"
	"storj.io/storj/satellite/metainfo"
)

const batchSize = 500
const objectsArgs = 14
const segmentsArgs = 11

type Entry struct {
	EncryptedKey string
	Metadata     string
	Index        int16
}

type ByKeyAndIndex []Entry

func (a ByKeyAndIndex) Len() int { return len(a) }
func (a ByKeyAndIndex) Less(i, j int) bool {
	if a[i].EncryptedKey == a[j].EncryptedKey {
		return a[i].Index < a[j].Index
	}
	return a[i].EncryptedKey < a[j].EncryptedKey
}
func (a ByKeyAndIndex) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type Migrator struct {
	PointerDBStr string
	PointerDB    metainfo.PointerDB
	Metabase     *metabase.Metabase

	ProjectID  uuid.UUID
	BucketName []byte

	BatchSize int

	ObjectsSQL     string
	Objects        []interface{}
	ObjectsCreated int

	SegmentsSQL     string
	Segments        []interface{}
	SegmentsCreated int
}

func NewMigrator(dbstr string, db metainfo.PointerDB, metabase *metabase.Metabase, projectID uuid.UUID, bucketName []byte) *Migrator {
	return &Migrator{
		PointerDBStr: dbstr,
		PointerDB:    db,
		Metabase:     metabase,

		ProjectID:  projectID,
		BucketName: bucketName,

		BatchSize: batchSize,

		ObjectsSQL: preparObjectsSQL(batchSize),
		Objects:    make([]interface{}, 0, batchSize*objectsArgs),

		SegmentsSQL: preparSegmentsSQL(batchSize),
		Segments:    make([]interface{}, 0, batchSize*segmentsArgs),
	}
}

// func (m *Migrator) MigrateBucket(ctx context.Context) error {
// 	path, err := metainfo.CreatePath(ctx, m.ProjectID, -1, m.BucketName, nil)
// 	if err != nil {
// 		return err
// 	}

// 	more := true
// 	lastKey := storage.Key{}
// 	pointer := &pb.Pointer{}
// 	key := path.Encode()
// 	for more {
// 		more, err = storage.ListV2Iterate(ctx, m.PointerDB, storage.ListOptions{
// 			Prefix:       storage.Key(key),
// 			StartAfter:   lastKey,
// 			Recursive:    true,
// 			Limit:        int(0),
// 			IncludeValue: true,
// 		}, func(ctx context.Context, item *storage.ListItem) error {
// 			err = pb.Unmarshal(item.Value, pointer)
// 			if err != nil {
// 				return err
// 			}

// 			encodedPath := item.Key
// 			if encodedPath[0] == '/' {
// 				encodedPath = encodedPath[1:]
// 			}

// 			err = m.insertObject(ctx, encodedPath, pointer)
// 			if err != nil {
// 				return err
// 			}

// 			lastKey = item.Key
// 			return nil
// 		})
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if len(m.Objects) != 0 {
// 		sql := preparObjectsSQL(len(m.Objects) / objectsArgs)
// 		err := m.Metabase.Exec(ctx, sql, m.Objects...)
// 		if err != nil {
// 			return err
// 		}
// 		m.ObjectsCreated += len(m.Objects) / objectsArgs
// 	}

// 	if len(m.Segments) != 0 {
// 		sql := preparSegmentsSQL(len(m.Segments) / segmentsArgs)
// 		err := m.Metabase.Exec(ctx, sql, m.Segments...)
// 		if err != nil {
// 			return err
// 		}
// 		m.SegmentsCreated += len(m.Segments) / segmentsArgs
// 	}

// 	return nil
// }

// MigrateBucket2 TODO
func (m *Migrator) MigrateBucket2(ctx context.Context) (err error) {
	conn, err := pgx.Connect(ctx, m.PointerDBStr)
	if err != nil {
		return fmt.Errorf("unable to connect %q: %w", m.PointerDBStr, err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close(ctx))
	}()

	query := ""
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("%s/s%d/%s", m.ProjectID.String(), i, m.BucketName)
		asHex := hex.EncodeToString([]byte(key))
		query += fmt.Sprintf(`OR (fullpath >= '\x%s2f' AND fullpath < '\x%s30')`, asHex, asHex)
	}

	projectIDHex := hex.EncodeToString([]byte(fmt.Sprintf("%s/l/%s", m.ProjectID.String(), m.BucketName)))
	sql := fmt.Sprintf(`
	SELECT fullpath, metadata FROM pathdata
	WHERE fullpath >= '\x%s2f' AND fullpath < '\x%s30'
	%s
	`, projectIDHex, projectIDHex, query)

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return err
	}

	defer func() { rows.Close() }()

	separator := []byte("/")

	entries := make([]Entry, 0, 10000)
	for rows.Next() {
		var path, metadata []byte
		err := rows.Scan(&path, &metadata)
		if err != nil {
			return err
		}

		segments := bytes.SplitN(path, separator, 4)
		entry := Entry{
			EncryptedKey: string(segments[3]),
			Metadata:     string(metadata),
		}

		if segments[1][0] == 'l' {
			entry.Index = -1
		} else {
			index, err := strconv.Atoi(string(segments[1][1:]))
			if err != nil {
				return err
			}
			entry.Index = int16(index)
		}

		entries = append(entries, entry)
	}

	sort.Sort(ByKeyAndIndex(entries))

	var streamID uuid.UUID

	pointer := &pb.Pointer{}
	streamMeta := &pb.StreamMeta{}

	for _, entry := range entries {
		err = pb.Unmarshal([]byte(entry.Metadata), pointer)
		if err != nil {
			return err
		}

		if entry.Index == -1 {
			streamID, err = uuid.New()
			if err != nil {
				return err
			}
			err = m.insertObject(ctx, streamID, []byte(entry.EncryptedKey), pointer)
			if err != nil {
				return err
			}
		} else {
			err = pb.Unmarshal(pointer.Metadata, streamMeta)
			if err != nil {
				return err
			}

			err = m.insertSegment(ctx, streamID, int64(entry.Index), pointer, streamMeta)
			if err != nil {
				return err
			}
		}
	}

	if len(m.Objects) != 0 {
		sql := preparObjectsSQL(len(m.Objects) / objectsArgs)
		err := m.Metabase.Exec(ctx, sql, m.Objects...)
		if err != nil {
			return err
		}
		m.ObjectsCreated += len(m.Objects) / objectsArgs
	}

	if len(m.Segments) != 0 {
		sql := preparSegmentsSQL(len(m.Segments) / segmentsArgs)
		err := m.Metabase.Exec(ctx, sql, m.Segments...)
		if err != nil {
			return err
		}
		m.SegmentsCreated += len(m.Segments) / segmentsArgs
	}
	return nil
}

func (m *Migrator) insertObject(ctx context.Context, streamID uuid.UUID, encryptedPath []byte, pointer *pb.Pointer) error {
	streamMeta := &pb.StreamMeta{}
	err := pb.Unmarshal(pointer.Metadata, streamMeta)
	if err != nil {
		return err
	}

	segmentsCount := streamMeta.NumberOfSegments
	if segmentsCount == 0 {
		return errors.New("unsupported case")
	}

	m.Objects = append(m.Objects,
		m.ProjectID, m.BucketName, encryptedPath, -1, streamID,
		pointer.CreationDate, pointer.ExpirationDate,
		metabase.Committed, segmentsCount,
		[]byte{}, pointer.Metadata, // TODO
		1000, 2000, // TODO
		33)

	if len(m.Objects)/objectsArgs >= m.BatchSize {
		err = m.sendObjects(ctx)
		if err != nil {
			return err
		}
	}

	err = m.insertSegment(ctx, streamID, segmentsCount-1, pointer, streamMeta)
	if err != nil {
		return err
	}
	return nil
}

func (m *Migrator) insertSegment(ctx context.Context, streamID uuid.UUID, segmentIndex int64, pointer *pb.Pointer, streamMeta *pb.StreamMeta) error {
	segmentPosition := metabase.SegmentPosition{
		Part:    0,
		Segment: uint32(segmentIndex),
	}

	rootPieceID := []byte{}
	if pointer.Remote != nil {
		rootPieceID = pointer.Remote.RootPieceId.Bytes()
	}

	if streamMeta == nil {
		streamMeta = &pb.StreamMeta{}
		err := pb.Unmarshal(pointer.Metadata, streamMeta)
		if err != nil {
			return err
		}
	}

	encryptedKey := []byte{}
	encryptedKeyNonce := []byte{}
	if streamMeta.LastSegmentMeta != nil {
		encryptedKey = streamMeta.LastSegmentMeta.EncryptedKey
		encryptedKeyNonce = streamMeta.LastSegmentMeta.KeyNonce
	}

	m.Segments = append(m.Segments,
		streamID, segmentPosition.Encode(),
		rootPieceID, encryptedKey, encryptedKeyNonce,
		1, 2,
		int32(pointer.SegmentSize),
		0,
		pointer.InlineSegment,
		metabase.NodeAliases{1}.Encode())

	if len(m.Segments)/segmentsArgs >= m.BatchSize {
		err := m.sendSegments(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) sendObjects(ctx context.Context) error {
	if len(m.Objects) == 0 {
		return nil
	}
	err := m.Metabase.Exec(ctx, m.ObjectsSQL, m.Objects...)
	if err != nil {
		return err
	}
	m.ObjectsCreated += len(m.Objects) / objectsArgs

	m.Objects = m.Objects[:0]

	return nil
}

func (m *Migrator) sendSegments(ctx context.Context) error {
	if len(m.Segments) == 0 {
		return nil
	}
	err := m.Metabase.Exec(ctx, m.SegmentsSQL, m.Segments...)
	if err != nil {
		return err
	}
	m.SegmentsCreated += len(m.Segments) / segmentsArgs

	m.Segments = m.Segments[:0]

	return nil
}

func preparObjectsSQL(batchSize int) string {
	sql := `
		INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata,
				total_encrypted_size, fixed_segment_size,
				encryption
		) VALUES
	`
	i := 1
	for i < batchSize*objectsArgs {
		sql += parameters(objectsArgs, i) + ","
		i += objectsArgs
	}
	return strings.TrimSuffix(sql, ",")
}

func preparSegmentsSQL(batchSize int) string {
	sql := `INSERT INTO segments (
		stream_id, segment_position, 
		root_piece_id, encrypted_key, encrypted_key_nonce,
		plain_offset, plain_size,
		encrypted_data_size,
		redundancy,
		inline_data, node_aliases
	) VALUES
	`
	i := 1
	for i < batchSize*segmentsArgs {
		sql += parameters(segmentsArgs, i) + ","
		i += segmentsArgs
	}

	return strings.TrimSuffix(sql, ",")
}

func parameters(args, index int) string {
	values := make([]string, args)
	for i := index; i < args+index; i++ {
		values[i-index] = "$" + strconv.Itoa(i)
	}
	return "(" + strings.Join(values, ",") + ")"
}
