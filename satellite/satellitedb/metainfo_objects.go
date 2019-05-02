// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

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
	objects.db.Create_Object(ctx,
		dbx.Object_BucketId(object.BucketID[:]),
		dbx.Object_EncryptedPath([]byte(object.EncryptedPath)),
		dbx.Object_Version(object.Version),
		dbx.Object_Status(int(object.Status)),

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

	return nil
}

func (objects *objects) Commit(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) (*metainfo.Object, error) {
	return nil, nil
}

func (objects *objects) Get(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) (*metainfo.Object, error) {
	return nil, nil
}

func (objects *objects) List(ctx context.Context, bucket uuid.UUID, opts metainfo.ListOptions) (metainfo.ObjectList, error) {
	return metainfo.ObjectList{}, nil
}

func (objects *objects) Delete(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) error {
	return nil
}

func (objects *objects) ListPartial(ctx context.Context, bucket uuid.UUID, opts metainfo.ListOptions) (metainfo.ObjectList, error) {
	return metainfo.ObjectList{}, nil
}

func (objects *objects) DeletePartial(ctx context.Context, bucket uuid.UUID, encryptedPath storj.Path, version uint32) error {
	return nil
}

func (segments *segments) Create(ctx context.Context, segment *metainfo.Segment) error {
	return nil
}

func (segments *segments) Commit(ctx context.Context, segment *metainfo.Segment) error {
	return nil
}

func (segments *segments) Delete(ctx context.Context, streamID uuid.UUID, segmentIndex int64) error {
	return nil
}

func (segments *segments) List(ctx context.Context, streamID uuid.UUID, segmentIndex int64, limit int64) ([]*metainfo.Segment, error) {
	return nil, nil
}
