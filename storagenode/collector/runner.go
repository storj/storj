// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
)

// RunOnce executes the collector only once.
type RunOnce struct {
	flatStore *pieces.PieceExpirationStore
	stop      *modular.StopTrigger
	log       *zap.Logger
	blobs     blobstore.Blobs
}

// NewRunnerOnce creates a new RunOnce.
func NewRunnerOnce(log *zap.Logger, peStore *pieces.PieceExpirationStore, blobs blobstore.Blobs, stop *modular.StopTrigger) RunOnce {
	return RunOnce{
		log:       log,
		flatStore: peStore,
		stop:      stop,
		blobs:     blobs,
	}
}

// Run implements Runner interface.
func (r RunOnce) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer r.stop.Cancel()
	monCounter := mon.Counter("collector_files_deleted")

	namespaces, err := r.blobs.ListNamespaces(ctx)
	if err != nil {
		return err
	}
	for _, ns := range namespaces {
		satelliteID, err := storj.NodeIDFromBytes(ns)
		if err != nil {
			r.log.Error("Invalid namespace", zap.Binary("namespace", ns), zap.Error(err))
			continue
		}
		expiredFiles, err := r.flatStore.GetExpiredFiles(ctx, satelliteID, time.Now())
		if err != nil {
			return err
		}
		if len(expiredFiles) == 0 {
			r.log.Info("No more expired files to handle, we are done!", zap.Stringer("satellite", satelliteID))
			continue
		}
		for _, next := range expiredFiles {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			r.log.Info("Deleting expired files", zap.String("file", next))

			problems := 0
			success := 0

			err = pieces.GetExpiredFromFile(ctx, next, func(id storj.PieceID, size uint64) {
				delErr := r.blobs.DeleteWithStorageFormat(ctx, blobstore.BlobRef{
					Namespace: ns,
					Key:       id.Bytes(),
				}, filestore.FormatV1, int64(size))
				if delErr != nil {
					r.log.Warn("Couldn't delete expired file", zap.Stringer("piece_id", id), zap.Error(delErr))
					problems++
				} else {
					success++
				}
				monCounter.Inc(1)

			})
			if err != nil || problems > 0 {
				r.log.Warn("Couldn't fully process expired file", zap.String("file", next), zap.Int("succes", success), zap.Int("failure", problems), zap.Error(err))
				continue
			}

			// process is cancelled, let's not delete the file, yet.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err = pieces.DeleteExpiredFile(ctx, next)
			if err != nil {
				r.log.Warn("Couldn't delete flat file", zap.String("file", next), zap.Error(err))
			}

			r.log.Info("Expired files deleted", zap.String("file", next), zap.Int("succes", success), zap.Int("failure", problems))
		}
	}

	return nil
}
