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
	db dbx.Methods
}

type segments struct {
	db dbx.Methods
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
	// mark object as committing with status partial
	// verify segments and collect info
	// mark object as committed
	return nil, nil
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
	// mark object as deleting with status committed
	// mark segments as being deleted to prevent further repairs
	return nil
}

func (objects *objects) FinishDelete(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version metainfo.ObjectVersion) error {
	// verify that all segments have been deleted
	// delete object
	return nil
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
	return nil
}

func (segments *segments) Commit(ctx context.Context, segment *metainfo.Segment) error {
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
		BucketId:      bucketID,
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
