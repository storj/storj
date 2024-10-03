// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"go.uber.org/zap"

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
		log:                 log,
		store:               store,
		trust:               trust,
		trashExpiryInterval: trashExpiryInterval,
		stop:                stop,
	}
}

// Run cleans up trashes.
func (t *TrashRunOnce) Run(ctx context.Context) error {
	for _, satellite := range t.trust.GetSatellites(ctx) {
		satellite := satellite
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
