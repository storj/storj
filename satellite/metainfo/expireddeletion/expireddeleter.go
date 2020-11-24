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

// RemoteSegment deletes the segment if it is expired.
func (ed *expiredDeleter) RemoteSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	return ed.deleteSegmentIfExpired(ctx, segment)
}

// InlineSegment deletes the segment if it is expired.
func (ed *expiredDeleter) InlineSegment(ctx context.Context, segment *metainfo.Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	return ed.deleteSegmentIfExpired(ctx, segment)
}

// Object returns nil because the expired deleter only cares about segments.
func (ed *expiredDeleter) Object(ctx context.Context, object *metainfo.Object) (err error) {
	return nil
}

func (ed *expiredDeleter) deleteSegmentIfExpired(ctx context.Context, segment *metainfo.Segment) error {
	if segment.Expired(time.Now()) {
		pointerBytes, err := pb.Marshal(segment.Pointer)
		if err != nil {
			return err
		}
		err = ed.metainfo.Delete(ctx, segment.Location.Encode(), pointerBytes)
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
