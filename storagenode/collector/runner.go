// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"context"
	"go.uber.org/zap"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"time"

	"storj.io/storj/shared/modular"
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
	before := time.Now()
	var next string
	namespaces, err := r.blobs.ListNamespaces(ctx)
	if err != nil {
		return err
	}
	for _, ns := range namespaces {
		var satelliteID storj.NodeID
		copy(satelliteID[:], ns)
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			next, before, err = r.flatStore.GetLatestExpired(ctx, satelliteID, before)
			if err != nil {
				return err
			}
			if next == "" {
				r.log.Info("No more expired pieces, we are done!")
				return nil
			}

			r.log.Info("Deleting expired files", zap.String("file", next))

			problems := 0
			success := 0

			err = pieces.GetExpiredFromFile(ctx, next, func(id storj.PieceID, size uint64) {
				err2 := r.blobs.DeleteWithStorageFormat(ctx, blobstore.BlobRef{
					Namespace: ns,
					Key:       id.Bytes(),
				}, filestore.FormatV1, int64(size))
				if err2 != nil {
					r.log.Warn("Couldn't delete expired file", zap.Stringer("piece_id", id), zap.Error(err2))
					problems++
				} else {
					success++
				}
				monCounter.Inc(1)

			})
			if err != nil || problems > 0 {
				r.log.Info("Couldn't fully process expired file", zap.String("file", next), zap.Error(err), zap.Int("succes", success), zap.Int("failure", problems))
				continue
			}

			// process is cancelled, let's not delete the file, yet.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err = pieces.DeleteExpiredFile(ctx, next)
			if err != nil {
				r.log.Warn("Couldn't delete flat piece", zap.String("file", next), zap.Error(err))
			}

			r.log.Info("Expired files deleted", zap.String("file", next), zap.Int("succes", success), zap.Int("failure", problems))
		}
	}

	return nil
}
