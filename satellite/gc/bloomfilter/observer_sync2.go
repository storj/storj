// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/bloomfilter"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
)

// SyncRetainInfo contains info needed for a storage node to retain important data and delete garbage data.
type SyncRetainInfo struct {
	mu     sync.Mutex
	Filter *bloomfilter.Filter
	Count  int
}

type SyncRetainInfos = map[storj.NodeID]*SyncRetainInfo

// SyncObserver implements a rangedloop observer to collect bloom filters for the garbage collection.
type SyncObserver2 struct {
	log     *zap.Logger
	config  Config
	overlay Overlay
	upload  *Upload

	// The following fields are reset for each loop.
	startTime       time.Time
	lastPieceCounts map[storj.NodeID]int64
	seed            byte

	mu sync.Mutex
	// LatestCreationTime will be used to set bloom filter CreationDate.
	// Because bloom filter service needs to be run against immutable database snapshot
	// we can set CreationDate for bloom filters as a latest segment CreatedAt value.
	latestCreationTime time.Time

	muRetainInfos sync.Mutex
	retainInfos   atomic.Pointer[SyncRetainInfos]
}

var _ (rangedloop.Observer) = (*Observer)(nil)

// NewSyncObserver creates a new instance of the gc rangedloop observer.
func NewSyncObserver2(log *zap.Logger, config Config, overlay Overlay) *SyncObserver2 {
	return &SyncObserver2{
		log:     log,
		overlay: overlay,
		upload:  NewUpload(log, config),
		config:  config,
	}
}

// Start is called at the beginning of each segment loop.
func (obs *SyncObserver2) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	switch {
	case obs.config.AccessGrant == "":
		return errs.New("Access Grant is not set")
	case obs.config.Bucket == "":
		return errs.New("Bucket is not set")
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
	infos := make(map[storj.NodeID]*SyncRetainInfo, len(lastPieceCounts))
	obs.retainInfos.Store(&infos)
	obs.latestCreationTime = time.Time{}
	obs.seed = bloomfilter.GenerateSeed()
	return nil
}

// Fork creates a Partial to build bloom filters over a chunk of all the segments.
func (obs *SyncObserver2) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	return obs, nil
}

// Join merges the bloom filters gathered by each Partial.
func (obs *SyncObserver2) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	return nil
}

// Finish uploads the bloom filters.
func (obs *SyncObserver2) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	retainInfos := *obs.retainInfos.Load()
	xs := make(map[storj.NodeID]*RetainInfo, len(retainInfos))
	for k, v := range retainInfos {
		xs[k] = &RetainInfo{
			Filter: v.Filter,
			Count:  v.Count,
		}
	}

	if err := obs.upload.UploadBloomFilters(ctx, obs.latestCreationTime, xs); err != nil {
		return err
	}
	obs.log.Debug("collecting bloom filters finished")
	return nil
}

// Process adds pieces to the bloom filter from remote segments.
func (obs *SyncObserver2) Process(ctx context.Context, segments []rangedloop.Segment) error {
	latestCreationTime := time.Time{}
	for _, segment := range segments {
		if segment.Inline() {
			continue
		}

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
func (obs *SyncObserver2) add(nodeID storj.NodeID, pieceID storj.PieceID) {
	retainInfos := obs.retainInfos.Load()
	info, ok := (*retainInfos)[nodeID]
	if !ok {
		info, ok = obs.recreateRetainInfos(nodeID)
		if !ok {
			return
		}
	}

	info.add(pieceID)
}

func (info *SyncRetainInfo) add(pieceID storj.PieceID) {
	info.mu.Lock()
	defer info.mu.Unlock()

	info.Filter.Add(pieceID)
	info.Count++
}

func (obs *SyncObserver2) recreateRetainInfos(nodeID storj.NodeID) (*SyncRetainInfo, bool) {
	// If we know how many pieces a node should be storing, use that number. Otherwise use default.
	numPieces := obs.config.InitialPieces
	pieceCounts, found := obs.lastPieceCounts[nodeID]
	if !found {
		// node was not in lastPieceCounts which means it was disqalified
		// and we won't generate bloom filter for it
		return nil, false
	}
	if pieceCounts > 0 {
		numPieces = pieceCounts
	}

	obs.muRetainInfos.Lock()
	defer obs.muRetainInfos.Unlock()

	retainInfos := obs.retainInfos.Load()

	// check whether some other goroutine already created the bloomfilter
	info, ok := (*retainInfos)[nodeID]
	if ok {
		// somebody beat the race
		return info, true
	}

	// clone the latest retainInfos
	xs := make(SyncRetainInfos, len(*retainInfos)+1)
	for k, v := range *retainInfos {
		xs[k] = v
	}

	hashCount, tableSize := bloomfilter.OptimalParameters(numPieces, obs.config.FalsePositiveRate, 2*memory.MiB)

	// limit size of bloom filter to ensure we are under the limit for RPC
	filter := bloomfilter.NewExplicit(obs.seed, hashCount, tableSize)
	info = &SyncRetainInfo{
		Filter: filter,
	}
	xs[nodeID] = info

	obs.retainInfos.Store(&xs)

	return info, true
}
