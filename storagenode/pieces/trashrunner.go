// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/trust"
)

// SupportEmptyTrashWithoutStat is an interface for blobstores that support emptying trash without calculating stats.
type SupportEmptyTrashWithoutStat interface {
	EmptyTrashWithoutStat(ctx context.Context, namespace []byte, trashedBefore time.Time) (err error)
}

// TrashRunOnce is a helper to run the trash cleaner only once.
type TrashRunOnce struct {
	log                 *zap.Logger
	trashExpiryInterval time.Duration
	blobs               blobstore.Blobs
	trust               *trust.Pool
	stop                *modular.StopTrigger
}

// NewTrashRunOnce creates a new TrashRunOnce.
func NewTrashRunOnce(log *zap.Logger, blobs blobstore.Blobs, trashExpiryInterval time.Duration, stop *modular.StopTrigger) *TrashRunOnce {
	return &TrashRunOnce{
		log:   log,
		blobs: blobs,
		// we intentionally ignore trust here, and use blobs.Listnamespace instead of trust.ListSatellites to avoid blocking the boltdb.
		trust:               nil,
		trashExpiryInterval: trashExpiryInterval,
		stop:                stop,
	}
}

// Run cleans up trashes.
func (t *TrashRunOnce) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	namespaces, err := t.blobs.ListNamespaces(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	for _, namespace := range namespaces {
		var satellite storj.NodeID
		copy(satellite[:], namespace)
		timeStart := time.Now()
		t.log.Info("emptying trash started", zap.Stringer("Satellite ID", satellite))
		trashedBefore := time.Now().Add(-t.trashExpiryInterval)
		if ws, ok := t.blobs.(SupportEmptyTrashWithoutStat); ok {
			err = ws.EmptyTrashWithoutStat(ctx, namespace, trashedBefore)
		} else {
			_, _, err = t.blobs.EmptyTrash(ctx, namespace, trashedBefore)
		}
		if err != nil {
			t.log.Error("emptying trash failed", zap.Error(err), zap.Stringer("satellite", satellite))
		} else {
			t.log.Info("emptying trash finished", zap.Stringer("satellite", satellite), zap.Duration("elapsed", time.Since(timeStart)))
		}

	}
	t.stop.Cancel()
	return nil
}
