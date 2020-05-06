// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package expireddeletion

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
)

var _ metainfo.Observer = (*expiredDeleter)(nil)

// expiredDeleter implements the metainfo loop observer interface for expired segment cleanup
//
// architecture: Observer
type expiredDeleter struct {
	log      *zap.Logger
	metainfo *metainfo.Service
}

// RemoteSegment deletes the segment if it is expired
func (ed *expiredDeleter) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx, path.Raw)(&err)

	return ed.deleteSegmentIfExpired(ctx, path, pointer)
}

// InlineSegment deletes the segment if it is expired
func (ed *expiredDeleter) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	defer mon.Task()(&ctx, path.Raw)(&err)

	return ed.deleteSegmentIfExpired(ctx, path, pointer)
}

// Object returns nil because the expired deleter only cares about segments
func (ed *expiredDeleter) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	return nil
}

func (ed *expiredDeleter) deleteSegmentIfExpired(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) error {
	// delete segment if expired
	if !pointer.ExpirationDate.IsZero() && pointer.ExpirationDate.Before(time.Now().UTC()) {
		pointerBytes, err := pb.Marshal(pointer)
		if err != nil {
			return err
		}
		err = ed.metainfo.Delete(ctx, path.Raw, pointerBytes)
		if storj.ErrObjectNotFound.Has(err) {
			// segment already deleted
			return nil
		} else if storage.ErrValueChanged.Has(err) {
			// segment was replaced
			return nil
		}
		return err
	}
	return nil
}
