// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
)

// Observer implements a rangedloop observer to collect bloom filters for the garbage collection.
//
// architecture: Observer
type Observer struct {
	log     *zap.Logger
	config  Config
	upload  *Upload
	overlay overlay.DB

	// The following fields are reset for each loop.
	startTime          time.Time
	lastPieceCounts    map[storj.NodeID]int64
	retainInfos        map[storj.NodeID]*RetainInfo
	latestCreationTime time.Time
	seed               byte
}

var _ (rangedloop.Observer) = (*Observer)(nil)

// NewObserver creates a new instance of the gc rangedloop observer.
func NewObserver(log *zap.Logger, config Config, overlay overlay.DB) *Observer {
	return &Observer{
		log:     log,
		overlay: overlay,
		upload:  NewUpload(log, config),
		config:  config,
	}
}

// Start is called at the beginning of each segment loop.
func (obs *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := obs.upload.CheckConfig(); err != nil {
		return err
	}

	obs.log.Debug("collecting bloom filters started")

	// load last piece counts from overlay db
	lastPieceCounts, err := obs.overlay.AllPieceCounts(ctx)
	if err != nil {
		obs.log.Error("error getting last piece counts", zap.Error(err))
		err = nil
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int64)
	}

	obs.startTime = startTime
	obs.lastPieceCounts = lastPieceCounts
	obs.retainInfos = make(map[storj.NodeID]*RetainInfo, len(lastPieceCounts))
	obs.latestCreationTime = time.Time{}
	obs.seed = bloomfilter.GenerateSeed()
	return nil
}

// Fork creates a Partial to build bloom filters over a chunk of all the segments.
func (obs *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: refactor PieceTracker after the segmentloop has been removed to
	// more closely match the rangedloop observer needs.
	pieceTracker := NewPieceTrackerWithSeed(obs.log.Named("gc observer"), obs.config, obs.lastPieceCounts, obs.seed)
	if err := pieceTracker.LoopStarted(ctx, segmentloop.LoopInfo{
		Started: obs.startTime,
	}); err != nil {
		return nil, err
	}
	return pieceTracker, nil
}

// Join merges the bloom filters gathered by each Partial.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)
	pieceTracker, ok := partial.(*PieceTracker)
	if !ok {
		return errs.New("expected %T but got %T", pieceTracker, partial)
	}

	// Update the count and merge the bloom filters for each node.
	for nodeID, retainInfo := range pieceTracker.RetainInfos {
		if existing, ok := obs.retainInfos[nodeID]; ok {
			existing.Count += retainInfo.Count
			if err := existing.Filter.AddFilter(retainInfo.Filter); err != nil {
				return err
			}
		} else {
			obs.retainInfos[nodeID] = retainInfo
		}
	}

	// Replace the latestCreationTime if the partial observed a later time.
	if obs.latestCreationTime.IsZero() || obs.latestCreationTime.Before(pieceTracker.LatestCreationTime) {
		obs.latestCreationTime = pieceTracker.LatestCreationTime
	}

	return nil
}

// Finish uploads the bloom filters.
func (obs *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err := obs.upload.UploadBloomFilters(ctx, obs.latestCreationTime, obs.retainInfos); err != nil {
		return err
	}
	obs.log.Debug("collecting bloom filters finished")
	return nil
}
