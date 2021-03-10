// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"os"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	proto "github.com/gogo/protobuf/proto"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/cmd/metainfo-migration/fastpb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/metabase"
)

const objectArgs = 14
const segmentArgs = 11

// EntryKey map key for object.
type EntryKey struct {
	Bucket string
	Key    metabase.ObjectKey
}

// Object represents object metadata.
type Object struct {
	StreamID               uuid.UUID
	CreationDate           time.Time
	ExpireAt               *time.Time
	EncryptedMetadata      []byte
	EncryptedMetadataKey   []byte
	EncryptedMetadataNonce []byte
	Encryption             int64
	TotalEncryptedSize     int64
	SegmentsRead           int64
	SegmentsExpected       int64
}

// Config initial settings for migrator.
type Config struct {
	PreGeneratedStreamIDs int
	ReadBatchSize         int
	WriteBatchSize        int
	WriteParallelLimit    int
	Nodes                 string
	InvalidObjectsFile    string
	NumberOfRetries       int
}

// Migrator defines metainfo migrator.
type Migrator struct {
	log           *zap.Logger
	pointerDBStr  string
	metabaseDBStr string
	config        Config

	objects  [][]interface{}
	segments [][]interface{}

	objectsSQL  string
	segmentsSQL string

	metabaseLimiter *sync2.Limiter
}

// NewMigrator creates new metainfo migrator.
func NewMigrator(log *zap.Logger, pointerDBStr, metabaseDBStr string, config Config) *Migrator {
	if config.ReadBatchSize == 0 {
		config.ReadBatchSize = defaultReadBatchSize
	}
	if config.WriteBatchSize == 0 {
		config.WriteBatchSize = defaultWriteBatchSize
	}
	if config.WriteParallelLimit == 0 {
		config.WriteParallelLimit = defaultWriteParallelLimit
	}
	if config.PreGeneratedStreamIDs == 0 {
		config.PreGeneratedStreamIDs = defaultPreGeneratedStreamIDs
	}
	if config.NumberOfRetries == 0 {
		config.NumberOfRetries = 5
	}
	return &Migrator{
		log:           log,
		pointerDBStr:  pointerDBStr,
		metabaseDBStr: metabaseDBStr,
		config:        config,

		objects:     make([][]interface{}, 0, config.WriteBatchSize),
		segments:    make([][]interface{}, 0, config.WriteBatchSize),
		objectsSQL:  prepareObjectsSQL(config.WriteBatchSize),
		segmentsSQL: prepareSegmentsSQL(config.WriteBatchSize),

		metabaseLimiter: sync2.NewLimiter(config.WriteParallelLimit),
	}
}

// MigrateProjects migrates all projects in pointerDB database.
func (m *Migrator) MigrateProjects(ctx context.Context) (err error) {
	m.log.Debug("Databases", zap.String("PointerDB", m.pointerDBStr), zap.String("MetabaseDB", m.metabaseDBStr))

	pointerDBConn, err := pgx.Connect(ctx, m.pointerDBStr)
	if err != nil {
		return errs.New("unable to connect %q: %w", m.pointerDBStr, err)
	}
	defer func() { err = errs.Combine(err, pointerDBConn.Close(ctx)) }()

	mb, err := metainfo.OpenMetabase(ctx, m.log.Named("metabase"), m.metabaseDBStr)
	if err != nil {
		return err
	}
	if err := mb.MigrateToLatest(ctx); err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, mb.Close()) }()

	aliasCache := metabase.NewNodeAliasCache(mb)

	config, err := pgxpool.ParseConfig(m.metabaseDBStr)
	if err != nil {
		return err
	}
	config.MaxConns = 10

	metabaseConn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return errs.New("unable to connect %q: %w", m.metabaseDBStr, err)
	}
	defer func() { metabaseConn.Close() }()

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
	segmentPosition := metabase.SegmentPosition{}
	object := Object{}
	location := metabase.ObjectLocation{}

	objects := make(map[EntryKey]Object)
	var currentProject uuid.UUID
	var fullpath, lastFullPath, metadata []byte
	var allObjects, allSegments, zombieSegments int64

	var invalidObjectsWriter *csv.Writer
	if m.config.InvalidObjectsFile != "" {
		objFile, err := os.Create(m.config.InvalidObjectsFile)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, objFile.Close()) }()

		invalidObjectsWriter = csv.NewWriter(objFile)
	} else {
		invalidObjectsWriter = csv.NewWriter(os.Stdout)
	}

	err = invalidObjectsWriter.Write([]string{"project_id", "bucket_name", "object_key", "stream_id", "expected_segments", "read_segments"})
	if err != nil {
		return err
	}

	start := time.Now()
	if m.config.Nodes != "" {
		err = m.aliasNodes(ctx, mb)
		if err != nil {
			return err
		}
	}

	lastCheck := time.Now()
	m.log.Info("Start generating StreamIDs", zap.Int("total", m.config.PreGeneratedStreamIDs))
	ids, err := generateStreamIDs(m.config.PreGeneratedStreamIDs)
	if err != nil {
		return err
	}
	m.log.Info("Finished generating StreamIDs", zap.Duration("took", time.Since(lastCheck)))

	m.log.Info("Start", zap.Time("time", start),
		zap.Int("readBatchSize", m.config.ReadBatchSize),
		zap.Int("writeBatchSize", m.config.WriteBatchSize),
		zap.Int("writeParallelLimit", m.config.WriteParallelLimit),
	)

	lastCheck = time.Now()
	for {
		hasResults := false
		err = func() error {
			var rows pgx.Rows
			if len(lastFullPath) == 0 {
				m.withRetry(ctx, func() (err error) {
					rows, err = pointerDBConn.Query(ctx, `SELECT fullpath, metadata FROM pathdata ORDER BY fullpath ASC LIMIT $1`, m.config.ReadBatchSize)
					return err
				})
			} else {
				m.withRetry(ctx, func() (err error) {
					rows, err = pointerDBConn.Query(ctx, `SELECT fullpath, metadata FROM pathdata WHERE fullpath > $1 ORDER BY fullpath ASC LIMIT $2`, lastFullPath, m.config.ReadBatchSize)
					return err
				})
			}

			defer func() { rows.Close() }()

			for rows.Next() {
				hasResults = true
				err = rows.Scan(&fullpath, &metadata)
				if err != nil {
					return err
				}

				lastFullPath = fullpath

				segmentKey, err := metabase.ParseSegmentKey(metabase.SegmentKey(fullpath))
				if err != nil {
					// we should skip such errors as it looks we can have outdated entries
					// in pointerdb like `project_id/l/bucket_name` without object key
					m.log.Warn("unable to parse segment key", zap.Error(err))
					continue
				}

				if !bytes.Equal(currentProject[:], segmentKey.ProjectID[:]) {
					if len(objects) != 0 {
						// TODO should we add such incomplete object into metabase?
						for key, object := range objects {
							err = invalidObjectsWriter.Write([]string{
								currentProject.String(),
								key.Bucket,
								hex.EncodeToString([]byte(key.Key)),
								object.StreamID.String(),
								strconv.FormatInt(object.SegmentsExpected, 10),
								strconv.FormatInt(object.SegmentsRead, 10),
							})
							if err != nil {
								return err
							}
						}
						invalidObjectsWriter.Flush()

						if err := invalidObjectsWriter.Error(); err != nil {
							return err
						}

						m.log.Warn("Object map should be empty after processing whole project", zap.String("ProjectID", currentProject.String()), zap.Int("Number of objects", len(objects)))
					}

					currentProject = segmentKey.ProjectID

					for b := range objects {
						delete(objects, b)
					}
				}
				err = proto.Unmarshal(metadata, pointer)
				if err != nil {
					return err
				}

				if allSegments != 0 && allSegments%1000000 == 0 {
					m.log.Info("Processed segments", zap.Int64("segments", allSegments), zap.Duration("took", time.Since(lastCheck)))
					lastCheck = time.Now()
				}

				key := EntryKey{
					Bucket: segmentKey.BucketName,
					Key:    segmentKey.ObjectKey,
				}

				// TODO:
				// * detect empty objects and insert only object
				if segmentKey.Position.Index == metabase.LastSegmentIndex {
					// process last segment, it contains information about object and segment metadata
					if len(ids) == 0 {
						return errs.New("not enough generated stream ids")
					}
					streamID := ids[0]
					err = proto.Unmarshal(pointer.Metadata, streamMeta)
					if err != nil {
						return err
					}
					// remove used ID
					ids = ids[1:]

					var expireAt *time.Time
					if !pointer.ExpirationDate.IsZero() {
						// because we are reusing Pointer struct using it directly can cause race
						copy := pointer.ExpirationDate
						expireAt = &copy
					}

					encryption, err := encodeEncryption(storj.EncryptionParameters{
						CipherSuite: storj.CipherSuite(streamMeta.EncryptionType),
						BlockSize:   streamMeta.EncryptionBlockSize,
					})
					if err != nil {
						return err
					}

					object.StreamID = streamID
					object.CreationDate = pointer.CreationDate
					object.ExpireAt = expireAt
					object.Encryption = encryption
					object.EncryptedMetadataKey = streamMeta.LastSegmentMeta.EncryptedKey
					object.EncryptedMetadataNonce = streamMeta.LastSegmentMeta.KeyNonce
					object.EncryptedMetadata = pointer.Metadata // TODO this needs to be striped to EncryptedStreamInfo

					object.SegmentsRead = 1
					object.TotalEncryptedSize = pointer.SegmentSize
					object.SegmentsExpected = streamMeta.NumberOfSegments

					// if object has only one segment then just insert it and don't put into map
					if streamMeta.NumberOfSegments == 1 {
						location.ProjectID = currentProject
						location.BucketName = key.Bucket
						location.ObjectKey = key.Key
						err = m.insertObject(ctx, metabaseConn, location, object)
						if err != nil {
							return err
						}
						allObjects++
					} else {
						objects[key] = object
					}

					segmentPosition.Index = uint32(streamMeta.NumberOfSegments - 1)
					err = m.insertSegment(ctx, metabaseConn, aliasCache, streamID, segmentPosition.Encode(), pointer, streamMeta.LastSegmentMeta)
					if err != nil {
						return err
					}
				} else {
					object, ok := objects[key]
					if !ok {
						// TODO verify if its possible that DB has zombie segments
						zombieSegments++
					} else {
						err = pb.Unmarshal(pointer.Metadata, segmentMeta)
						if err != nil {
							return err
						}

						segmentPosition.Index = segmentKey.Position.Index
						err = m.insertSegment(ctx, metabaseConn, aliasCache, object.StreamID, segmentPosition.Encode(), pointer, segmentMeta)
						if err != nil {
							return err
						}

						object.SegmentsRead++
						object.TotalEncryptedSize += pointer.SegmentSize
						if object.SegmentsRead == object.SegmentsExpected {
							location.ProjectID = currentProject
							location.BucketName = key.Bucket
							location.ObjectKey = key.Key
							err = m.insertObject(ctx, metabaseConn, location, object)
							if err != nil {
								return err
							}
							allObjects++

							delete(objects, key)
						} else {
							objects[key] = object
						}
					}
				}

				allSegments++
			}

			return rows.Err()
		}()
		if err != nil {
			return err
		}
		if !hasResults {
			break
		}
	}

	err = m.flushObjects(ctx, metabaseConn)
	if err != nil {
		return err
	}
	err = m.flushSegments(ctx, metabaseConn)
	if err != nil {
		return err
	}

	m.metabaseLimiter.Wait()

	m.log.Info("Finished", zap.Int64("objects", allObjects), zap.Int64("segments", allSegments), zap.Int64("invalid", zombieSegments), zap.Duration("total", time.Since(start)))

	return nil
}

func (m *Migrator) insertObject(ctx context.Context, conn *pgxpool.Pool, location metabase.ObjectLocation, object Object) error {
	m.objects = append(m.objects, []interface{}{
		location.ProjectID, location.BucketName, []byte(location.ObjectKey), 1, object.StreamID,
		object.CreationDate, object.ExpireAt,
		metabase.Committed, object.SegmentsRead,
		object.EncryptedMetadata, object.EncryptedMetadataKey, object.EncryptedMetadataNonce,
		object.TotalEncryptedSize,
		object.Encryption,
	})

	if len(m.objects) >= m.config.WriteBatchSize {
		err := m.flushObjects(ctx, conn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) insertSegment(ctx context.Context, conn *pgxpool.Pool, aliasCache *metabase.NodeAliasCache, streamID uuid.UUID, position uint64, pointer *fastpb.Pointer, segmentMeta *fastpb.SegmentMeta) (err error) {
	var rootPieceID storj.PieceID
	var remotePieces metabase.Pieces
	var redundancy int64
	if pointer.Type == fastpb.Pointer_REMOTE && pointer.Remote != nil {
		rootPieceID = pointer.Remote.RootPieceId
		redundancy, err = encodeRedundancy(pointer.Remote.Redundancy)
		if err != nil {
			return err
		}

		for _, remotePiece := range pointer.Remote.RemotePieces {
			remotePieces = append(remotePieces, metabase.Piece{
				Number:      uint16(remotePiece.PieceNum),
				StorageNode: remotePiece.NodeId,
			})
		}
	}

	pieces, err := aliasCache.ConvertPiecesToAliases(ctx, remotePieces)
	if err != nil {
		return err
	}

	sort.Slice(pieces, func(i, j int) bool {
		return pieces[i].Number < pieces[j].Number
	})

	m.segments = append(m.segments, []interface{}{
		streamID, position,
		rootPieceID,
		segmentMeta.EncryptedKey, segmentMeta.KeyNonce,
		pointer.SegmentSize, 0, 0,
		redundancy,
		pointer.InlineSegment, pieces,
	})

	if len(m.segments) >= m.config.WriteBatchSize {
		err = m.flushSegments(ctx, conn)
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeEncryption(params storj.EncryptionParameters) (int64, error) {
	var bytes [8]byte
	bytes[0] = byte(params.CipherSuite)
	binary.LittleEndian.PutUint32(bytes[1:], uint32(params.BlockSize))
	return int64(binary.LittleEndian.Uint64(bytes[:])), nil
}

func encodeRedundancy(redundancy *fastpb.RedundancyScheme) (int64, error) {
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

func (m *Migrator) flushObjects(ctx context.Context, conn *pgxpool.Pool) error {
	if len(m.objects) == 0 {
		return nil
	}

	objectsSQL := m.objectsSQL
	if len(m.objects) < m.config.WriteBatchSize {
		objectsSQL = prepareObjectsSQL(len(m.objects))
	}

	// TODO make predefined instance for that
	params := []interface{}{}
	for _, object := range m.objects {
		params = append(params, object...)
	}

	m.metabaseLimiter.Go(ctx, func() {
		params := params
		m.withRetry(ctx, func() error {
			_, err := conn.Exec(ctx, objectsSQL, params...)
			return err
		})
	})

	m.objects = m.objects[:0]
	return nil
}

func (m *Migrator) flushSegments(ctx context.Context, conn *pgxpool.Pool) error {
	if len(m.segments) == 0 {
		return nil
	}

	segmentsSQL := m.segmentsSQL
	if len(m.segments) < m.config.WriteBatchSize {
		segmentsSQL = prepareSegmentsSQL(len(m.segments))
	}

	// TODO make predefined instance for that
	params := make([]interface{}, 0, len(m.segments)*segmentArgs)
	for _, segment := range m.segments {
		params = append(params, segment...)
	}

	m.metabaseLimiter.Go(ctx, func() {
		params := params
		m.withRetry(ctx, func() error {
			_, err := conn.Exec(ctx, segmentsSQL, params...)
			return err
		})
	})

	m.segments = m.segments[:0]
	return nil
}

func prepareObjectsSQL(batchSize int) string {
	sql := `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_metadata_nonce,
			total_encrypted_size,
			encryption
		) VALUES
	`
	i := 1
	for i < batchSize*objectArgs {
		sql += parameters(objectArgs, i) + ","
		i += objectArgs
	}
	return strings.TrimSuffix(sql, ",")
}

func prepareSegmentsSQL(batchSize int) string {
	sql := `INSERT INTO segments (
		stream_id, position,
		root_piece_id, encrypted_key, encrypted_key_nonce,
		encrypted_size, plain_offset, plain_size,
		redundancy,
		inline_data, remote_alias_pieces
	) VALUES
	`
	i := 1
	for i < batchSize*segmentArgs {
		sql += parameters(segmentArgs, i) + ","
		i += segmentArgs
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

func generateStreamIDs(numberOfIDs int) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, numberOfIDs)
	var err error
	for i := 0; i < len(ids); i++ {
		ids[i], err = uuid.New()
		if err != nil {
			return []uuid.UUID{}, err
		}
	}

	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i][:], ids[j][:]) == -1
	})
	return ids, nil
}

func (m *Migrator) aliasNodes(ctx context.Context, mb metainfo.MetabaseDB) error {
	start := time.Now()
	m.log.Info("Start aliasing nodes")
	file, err := os.Open(m.config.Nodes)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, file.Close()) }()

	scanner := bufio.NewScanner(file)
	nodes := make([]storj.NodeID, 0, 30000)
	for scanner.Scan() {
		line := scanner.Text()
		decoded, err := hex.DecodeString(line)
		if err != nil {
			m.log.Error("unable decode node id", zap.String("value", line), zap.Error(err))
			continue
		}
		node, err := storj.NodeIDFromBytes(decoded)
		if err != nil {
			m.log.Error("unable create node id", zap.String("value", line), zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	// batch is used because we had issue with CRDB to put all nodes in one insert
	batch := 1000
	for len(nodes) > 0 {
		if len(nodes) < batch {
			batch = len(nodes)
		}

		err = mb.EnsureNodeAliases(ctx, metabase.EnsureNodeAliases{
			Nodes: nodes[:batch],
		})
		if err != nil {
			return err
		}
		nodes = nodes[batch:]
		m.log.Info("Left to insert", zap.Int("nodes", len(nodes)))
	}
	m.log.Info("Finished aliasing nodes", zap.Duration("took", time.Since(start)))
	return nil
}

func (m *Migrator) withRetry(ctx context.Context, fn func() error) {
	var err error
	for i := 0; i < m.config.NumberOfRetries; i++ {
		err = fn()
		if err != nil {
			m.log.Error("error occur", zap.Int("retry", i), zap.Error(err))

			if !sync2.Sleep(ctx, 3*time.Second) {
				m.log.Fatal("context error", zap.Error(ctx.Err()))
			}
			continue
		}
		return
	}

	// make no sense to continue if even single query to DB fails
	m.log.Fatal("query failed after retries", zap.Error(err))
}
