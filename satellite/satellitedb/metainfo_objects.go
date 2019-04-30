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
