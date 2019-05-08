// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var _ metainfo.Objects = (*objects)(nil)
var _ metainfo.Segments = (*segments)(nil)

type objects struct {
	db *dbx.DB
}

type segments struct {
	db *dbx.DB
}

func (objects *objects) Create(ctx context.Context, object *metainfo.Object) error {
	_, err := objects.db.Create_Object(ctx,
		dbx.Object_BucketId(object.BucketID[:]),
		dbx.Object_EncryptedPath([]byte(object.EncryptedPath)),
		dbx.Object_Version(int64(object.Version)),
		dbx.Object_Status(int(metainfo.Partial)),

		dbx.Object_StreamId(object.StreamID[:]),
		dbx.Object_EncryptedMetadata(object.EncryptedMetadata),

		dbx.Object_TotalSize(object.TotalSize),
		dbx.Object_InlineSize(object.InlineSize),
		dbx.Object_RemoteSize(object.RemoteSize),

		dbx.Object_CreatedAt(object.CreatedAt),
		dbx.Object_ExpiresAt(object.ExpiresAt),

		dbx.Object_FixedSegmentSize(object.FixedSegmentSize),

		dbx.Object_EncryptionCipherSuite(int(object.Encryption.CipherSuite)),
		dbx.Object_EncryptionBlockSize(int(object.Encryption.BlockSize)),

		dbx.Object_RedundancyAlgorithm(int(object.Redundancy.Algorithm)),
		dbx.Object_RedundancyShareSize(int(object.Redundancy.ShareSize)),
		dbx.Object_RedundancyRequiredShares(int(object.Redundancy.RequiredShares)),
		dbx.Object_RedundancyRepairShares(int(object.Redundancy.RepairShares)),
		dbx.Object_RedundancyOptimalShares(int(object.Redundancy.OptimalShares)),
		dbx.Object_RedundancyTotalShares(int(object.Redundancy.TotalShares)),
	)
	return err
}

func (objects *objects) Commit(ctx context.Context, object *metainfo.Object) (*metainfo.Object, error) {
	// TODO: remove any other irrelevant fields from the argument

	_, err := objects.db.ExecContext(ctx, objects.db.Rebind(`
	BEGIN TRANSACTION;
		DO $$
		DECLARE
			target_id        uuid;
		
			min_segment_size bigint;
			max_segment_size bigint;
		
			var_inline_size      bigint;
			var_remote_size      bigint;
			var_fixed_segment_size bigint = 0;
		BEGIN
			UPDATE objects
				SET status = 'committing'::object_status
				WHERE bucket_id	= 'core'::bytea AND encrypted_path = 'alpha/something'::bytea
				RETURNING stream_id INTO target_id;
		
			SELECT
				sum(CASE WHEN segment_size IS NULL THEN segment_size ELSE 0 END),
				sum(CASE WHEN segment_size IS NOT NULL THEN segment_size ELSE 0 END),
				min(segment_size),
				max(segment_size)
				INTO var_inline_size, var_remote_size, min_segment_size, max_segment_size
				FROM segments
				WHERE stream_id = target_id;
			
			IF min_segment_size = max_segment_size THEN
				var_fixed_segment_size = min_segment_size;
			END IF;
		
			-- doesn't work
			-- WITH ordered_segments AS (
			-- 	SELECT 
			-- ) UPDATE segments SET segment_index = new_segment_index;
		
			UPDATE segments
				SET segments.segment_index = segments.new_segment_index
				FROM (
				) segments;
				
				segment_index = ROW_NUMBER,
				status = 'committed'::segment_status
				WHERE stream_id = target_id;
			
			UPDATE objects SET
				status = 'committed'::object_status,
				fixed_segment_size = var_fixed_segment_size,
				total_size  = var_inline_size + var_remote_size,
				inline_size = var_inline_size,
				remote_size = var_remote_size
				WHERE bucket_id	= 'core'::bytea AND encrypted_path = 'alpha/something'::bytea;
		END $$;
	COMMIT;
	`), object.BucketID[:], object.EncryptedPath, object.Version)
	return nil, err

	/*
		result, err := objects.db.Update_Object_By_BucketId_And_EncryptedPath_And_Version_And_Status(ctx,
			dbx.Object_BucketId(object.BucketID[:]),
			dbx.Object_EncryptedPath([]byte(object.EncryptedPath)),
			dbx.Object_Version(int64(object.Version)),
			dbx.Object_Status(int(metainfo.Partial)),
			dbx.Object_Update_Fields{
				Status: dbx.Object_Status(int(metainfo.Committing)),
			},
		)
		if err != nil {
			return nil, err
		}

		streamID := dbx.Segment_StreamId(result.StreamId)
		totalSize := int64(0)
		inlineSize := int64(0)
		remoteSize := int64(0)
		fixedSegmentSize := -1

		dbxSegments, err := objects.db.Limited_Segment_By_StreamId_And_SegmentIndex_GreaterOrEqual_OrderBy_Asc_SegmentIndex(ctx, streamID, dbx.Segment_SegmentIndex(0), 10, 0)
		if err == sql.ErrNoRows {
			// abort, or allow empty file?
			return nil, err
		}
		if err != nil {
			// undo object status
			return nil, err
		}

		for len(dbxSegments) > 0 {
			var lastIndex uint64
			for _, dbxSegment := range dbxSegments {
				lastIndex = dbxSegment.SegmentIndex
				if dbxSegment.SegmentSize < 0 {
					// abort
					return nil, err
				}

				totalSize += int64(len(dbxSegment.EncryptedInlineData))
				inlineSize += int64(len(dbxSegment.EncryptedInlineData))
				if len(dbxSegment.Nodes) > 0 {
					totalSize += dbxSegment.SegmentSize
					remoteSize += dbxSegment.SegmentSize
			}

			dbxSegments, err = objects.db.Limited_Segment_By_StreamId_And_SegmentIndex_Greater_OrderBy_Asc_SegmentIndex(ctx, streamID, dbx.Segment_SegmentIndex(lastIndex), 1, 0)
		}
		if err == sql.ErrNoRows {
			err = nil
		}
		if err != nil {
			// undo object status
			return nil, err
		}

		result, err := objects.db.Update_Object_By_BucketId_And_EncryptedPath_And_Version_And_Status(ctx,
			dbx.Object_BucketId(object.BucketID[:]),
			dbx.Object_EncryptedPath([]byte(object.EncryptedPath)),
			dbx.Object_Version(int64(object.Version)),
			dbx.Object_Status(int(metainfo.Partial)),
			dbx.Object_Update_Fields{
				Status: dbx.Object_Status(int(metainfo.Committing)),
			},
		)
		if err != nil {
			return nil, err
		}

		// mark object as committing with status partial
		// verify segments and collect info
		// mark object as committed
		return nil, nil
	*/
}

func (objects *objects) Get(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) (*metainfo.Object, error) {
	if version == metainfo.LastObjectVersion {
		dbxObject, err := objects.db.Get_Object_By_BucketId_And_EncryptedPath_And_Status_OrderBy_Desc_Version(ctx,
			dbx.Object_BucketId(bucket[:]),
			dbx.Object_EncryptedPath([]byte(encryptedPath)),
			dbx.Object_Status(int(metainfo.Committed)),
		)
		if err != nil {
			return nil, err
		}
		return objectFromDBX(dbxObject)
	}

	dbxObject, err := objects.db.Get_Object_By_BucketId_And_EncryptedPath_And_Version_And_Status(ctx,
		dbx.Object_BucketId(bucket[:]),
		dbx.Object_EncryptedPath([]byte(encryptedPath)),
		dbx.Object_Version(int64(version)),
		dbx.Object_Status(int(metainfo.Committed)),
	)
	if err != nil {
		return nil, err
	}
	return objectFromDBX(dbxObject)
}

func (objects *objects) List(ctx context.Context, bucket uuid.UUID, opts metainfo.ListOptions) (metainfo.ObjectList, error) {
	return metainfo.ObjectList{}, nil
}

func (objects *objects) StartDelete(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) error {
	_, err := objects.db.ExecContext(ctx, objects.db.Rebind(`
	BEGIN TRANSACTION;
		-- get the relevant information
		obj := UPDATE status = deleting
			WHERE bucket_id = ? AND encrypted_path = ? AND version = ? AND
				status = committed;

		-- update segment indices
		UPDATE segments SET status = deleting WHERE stream_id = obj.stream_id;
	COMMIT;
	`))
	return nil, err
}

func (objects *objects) FinishDelete(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) error {
	_, err := objects.db.ExecContext(ctx, objects.db.Rebind(`
	BEGIN TRANSACTION;
		-- update segment indices
		select segments WHERE stream_id = obj.stream_id;
		-- ensure that there are none
		
		-- get the relevant information
		obj := DELETE objects WHERE bucket_id = ? AND encrypted_path = ? AND version = ? AND status = deleting;
	COMMIT;
	`))
	return nil, err
}

func (objects *objects) GetPartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) (*metainfo.Object, error) {
	// this currently returns non-committed
	if version == metainfo.LastObjectVersion {
		dbxObject, err := objects.db.Get_Object_By_BucketId_And_EncryptedPath_And_Status_Not_OrderBy_Desc_Version(ctx,
			dbx.Object_BucketId(bucket[:]),
			dbx.Object_EncryptedPath([]byte(encryptedPath)),
			dbx.Object_Status(int(metainfo.Committed)),
		)
		if err != nil {
			return nil, err
		}
		return objectFromDBX(dbxObject)
	}

	dbxObject, err := objects.db.Get_Object_By_BucketId_And_EncryptedPath_And_Version_And_Status_Not(ctx,
		dbx.Object_BucketId(bucket[:]),
		dbx.Object_EncryptedPath([]byte(encryptedPath)),
		dbx.Object_Version(int64(version)),
		dbx.Object_Status(int(metainfo.Committed)),
	)
	if err != nil {
		return nil, err
	}
	return objectFromDBX(dbxObject)
}

func (objects *objects) ListPartial(ctx context.Context, bucket uuid.UUID, opts metainfo.ListOptions) (metainfo.ObjectList, error) {
	return metainfo.ObjectList{}, nil
}

func (objects *objects) DeletePartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) error {
	return nil
}

func (segments *segments) Create(ctx context.Context, segment *metainfo.Segment) error {
	// insert new segment with
	//   segment_size = -1
	return nil
}

func (segments *segments) Commit(ctx context.Context, segment *metainfo.Segment) error {
	// verify against segment size and update missing fields
	return nil
}

func (objects *objects) StartDeletePartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) error {
	// mark object as deleting with status committed
	// mark segments as being deleted to prevent further repairs
	return nil
}

func (objects *objects) FinishDeletePartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) error {
	// verify that all segments have been deleted
	// delete object
	return nil
}

func (segments *segments) List(ctx context.Context, streamID uuid.UUID, segmentIndex int64, limit int64) ([]*metainfo.Segment, error) {
	return nil, nil
}

// objectFromDBX is used for creating object entity from autogenerated dbx.Object struct
func objectFromDBX(object *dbx.Object) (*metainfo.Object, error) {
	if object == nil {
		return nil, errs.New("object parameter is nil")
	}

	bucketID, err := bytesToUUID(object.BucketId)
	if err != nil {
		return nil, err
	}

	streamID, err := bytesToUUID(object.StreamId)
	if err != nil {
		return nil, err
	}

	return &metainfo.Object{
		BucketID:      bucketID,
		EncryptedPath: storj.Path(object.EncryptedPath),
		Version:       metainfo.ObjectVersion(object.Version),
		Status:        metainfo.ObjectStatus(object.Status),

		StreamID:          streamID,
		EncryptedMetadata: object.EncryptedMetadata,

		TotalSize:  object.TotalSize,
		InlineSize: object.InlineSize,
		RemoteSize: object.RemoteSize,

		CreatedAt: object.CreatedAt,
		ExpiresAt: object.ExpiresAt,

		Encryption: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(object.EncryptionCipherSuite),
			BlockSize:   int32(object.EncryptionBlockSize),
		},
		Redundancy: storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(object.RedundancyAlgorithm),
			ShareSize:      int32(object.RedundancyShareSize),
			RequiredShares: int16(object.RedundancyRequiredShares),
			RepairShares:   int16(object.RedundancyRepairShares),
			OptimalShares:  int16(object.RedundancyOptimalShares),
			TotalShares:    int16(object.RedundancyTotalShares),
		},
	}, nil
}
