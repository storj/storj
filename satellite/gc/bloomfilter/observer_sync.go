// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/shared/nodeidmap"
)

// SyncObserver implements a rangedloop observer to collect bloom filters for the garbage collection.
type SyncObserver struct {
	log     *zap.Logger
	config  Config
	overlay Overlay
	upload  *Upload

	// The following fields are reset for each loop.
	startTime       time.Time
	lastPieceCounts map[storj.NodeID]int64
	seed            byte

	mu          sync.Mutex
	retainInfos nodeidmap.Map[*RetainInfo]
	// LatestCreationTime will be used to set bloom filter CreationDate.
	// Because bloom filter service needs to be run against immutable database snapshot
	// we can set CreationDate for bloom filters as a latest segment CreatedAt value.
	latestCreationTime time.Time

	forcedTableSize int

	inlineCount, remoteCount int
}

var _ (rangedloop.Observer) = (*SyncObserver)(nil)
var _ (rangedloop.Partial) = (*SyncObserver)(nil)

// NewSyncObserver creates a new instance of the gc rangedloop observer.
func NewSyncObserver(log *zap.Logger, config Config, overlay Overlay) *SyncObserver {
	return &SyncObserver{
		log:     log,
		overlay: overlay,
		upload:  NewUpload(log, config),
		config:  config,
	}
}

// Start is called at the beginning of each segment loop.
func (obs *SyncObserver) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := obs.upload.CheckConfig(); err != nil {
		return err
	}

	obs.log.Debug("collecting bloom filters started")

	// load last piece counts from overlay db
	lastPieceCounts, err := obs.overlay.ActiveNodesPieceCounts(ctx)
	if err != nil {
		obs.log.Error("error getting last piece counts", zap.Error(err))
		err = nil
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int64)
	}

	obs.startTime = startTime
	obs.lastPieceCounts = lastPieceCounts
	obs.retainInfos = nodeidmap.MakeSized[*RetainInfo](len(lastPieceCounts))
	obs.latestCreationTime = time.Time{}
	obs.seed = bloomfilter.GenerateSeed()
	return nil
}

// Fork creates a Partial to build bloom filters over a chunk of all the segments.
func (obs *SyncObserver) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	return obs, nil
}

// Join merges the bloom filters gathered by each Partial.
func (obs *SyncObserver) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	return nil
}

// Finish uploads the bloom filters.
func (obs *SyncObserver) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := obs.upload.UploadBloomFilters(ctx, obs.latestCreationTime, obs.retainInfos); err != nil {
		return err
	}

	obs.log.Info("collecting bloom filters finished",
		zap.Int("inline segments", obs.inlineCount),
		zap.Int("remote segments", obs.remoteCount))

	return nil
}

// TestingRetainInfos returns retain infos collected by observer.
func (obs *SyncObserver) TestingRetainInfos() MinimalRetainInfoMap {
	return obs.retainInfos
}

// TestingForceTableSize sets a fixed size for tables. Used for testing.
func (obs *SyncObserver) TestingForceTableSize(size int) {
	obs.forcedTableSize = size
}

// Process adds pieces to the bloom filter from remote segments.
func (obs *SyncObserver) Process(ctx context.Context, segments []rangedloop.Segment) error {
	latestCreationTime := time.Time{}
	for _, segment := range segments {
		if segment.Inline() {
			obs.mu.Lock()
			obs.inlineCount++
			obs.mu.Unlock()
			continue
		}

		obs.mu.Lock()
		obs.remoteCount++
		obs.mu.Unlock()

		// sanity check to detect if loop is not running against live database
		if segment.CreatedAt.After(obs.startTime) {
			obs.log.Error("segment created after loop started", zap.Stringer("StreamID", segment.StreamID),
				zap.Time("loop started", obs.startTime),
				zap.Time("segment created", segment.CreatedAt))
			return errs.New("segment created after loop started")
		}

		if latestCreationTime.Before(segment.CreatedAt) {
			latestCreationTime = segment.CreatedAt
		}

		deriver := segment.RootPieceID.Deriver()
		for _, piece := range segment.Pieces {
			pieceID := deriver.Derive(piece.StorageNode, int32(piece.Number))
			obs.add(piece.StorageNode, pieceID)
		}
	}

	obs.mu.Lock()
	defer obs.mu.Unlock()

	if obs.latestCreationTime.Before(latestCreationTime) {
		obs.latestCreationTime = latestCreationTime
	}

	return nil
}

// add adds a pieceID to the relevant node's RetainInfo.
func (obs *SyncObserver) add(nodeID storj.NodeID, pieceID storj.PieceID) {
	obs.mu.Lock()
	defer obs.mu.Unlock()

	info, ok := obs.retainInfos.Load(nodeID)
	if !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		numPieces := obs.config.InitialPieces
		if pieceCounts, found := obs.lastPieceCounts[nodeID]; found {
			if pieceCounts > 0 {
				numPieces = pieceCounts
			}
		} else {
			// node was not in lastPieceCounts which means it was disqalified
			// and we won't generate bloom filter for it
			return
		}

		hashCount, tableSize := bloomfilter.OptimalParameters(numPieces, obs.config.FalsePositiveRate, obs.config.MaxBloomFilterSize)
		// limit size of bloom filter to ensure we are under the limit for RPC
		if obs.forcedTableSize > 0 {
			tableSize = obs.forcedTableSize
		}
		filter := bloomfilter.NewExplicit(obs.seed, hashCount, tableSize)
		info = &RetainInfo{
			Filter: filter,
		}
		obs.retainInfos.Store(nodeID, info)
	}

	info.Filter.Add(pieceID)
	info.Count++
}
