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
	"storj.io/storj/storagenode/trust"
)

// TrashRunOnce is a helper to run the trash cleaner only once.
type TrashRunOnce struct {
	log                 *zap.Logger
	trashExpiryInterval time.Duration
	store               *Store
	trust               *trust.Pool
	stop                *modular.StopTrigger
}

// NewTrashRunOnce creates a new TrashRunOnce.
func NewTrashRunOnce(log *zap.Logger, trust *trust.Pool, store *Store, trashExpiryInterval time.Duration, stop *modular.StopTrigger) *TrashRunOnce {
	return &TrashRunOnce{
		log:   log,
		store: store,
		// we intentionally ignore trust here, and use blobs.Listnamespace instead of trust.ListSatellites to avoid blocking the boltdb.
		trust:               nil,
		trashExpiryInterval: trashExpiryInterval,
		stop:                stop,
	}
}

// Run cleans up trashes.
func (t *TrashRunOnce) Run(ctx context.Context) error {
	namespaces, err := t.store.blobs.ListNamespaces(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	for _, namespace := range namespaces {
		var satellite storj.NodeID
		copy(satellite[:], namespace)
		timeStart := time.Now()
		t.log.Info("emptying trash started", zap.Stringer("Satellite ID", satellite))
		trashedBefore := time.Now().Add(-t.trashExpiryInterval)
		err := t.store.EmptyTrash(ctx, satellite, trashedBefore)
		if err != nil {
			t.log.Error("emptying trash failed", zap.Error(err))
		} else {
			t.log.Info("emptying trash finished", zap.Stringer("Satellite ID", satellite), zap.Duration("elapsed", time.Since(timeStart)))
		}
	}
	t.stop.Cancel()
	return nil
}
