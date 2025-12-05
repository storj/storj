// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/bloomfilter"
)

type concurrentRetainInfo struct {
	mu   sync.Mutex
	info *RetainInfo
}

type concurrentRetainInfos struct {
	m sync.Map
}

// IsEmpty implements MinimalRetainInfoMap.
func (c *concurrentRetainInfos) IsEmpty() bool {
	empty := true
	c.m.Range(func(key, value interface{}) bool {
		empty = false
		return false
	})
	return empty
}

// Load implements MinimalRetainInfoMap.
func (c *concurrentRetainInfos) Load(nodeID storj.NodeID) (info *RetainInfo, ok bool) {
	value, ok := c.m.Load(nodeID)
	if !ok {
		return nil, false
	}
	return value.(*concurrentRetainInfo).info, true
}

// Range implements MinimalRetainInfoMap.
func (c *concurrentRetainInfos) Range(f func(nodeID storj.NodeID, info *RetainInfo) bool) {
	c.m.Range(func(key, value any) bool {
		info := value.(*concurrentRetainInfo).info
		if info == nil {
			// We will inevitably have nil values in the map because we
			// always add the locking information for storage nodes,
			// even those we will not generate bloom filters for. In
			// this case, we iterate further and ignore the nil value.
			return true
		}
		return f(key.(storj.NodeID), info)
	})
}

// SyncObserverV2 implements collects bloom filters for garbage
// collection.
type SyncObserverV2 struct {
	log     *zap.Logger
	config  Config
	overlay Overlay
	upload  *Upload

	retainInfos     *concurrentRetainInfos
	forcedTableSize int

	// The following fields are reset for each loop.
	startTime       time.Time
	lastPieceCounts map[storj.NodeID]int64
	seed            byte

	inlineCount, remoteCount atomic.Uint64

	// LatestCreationTime will be used to set bloom filter CreationDate.
	mu                 sync.Mutex
	latestCreationTime time.Time
}

var (
	_ (rangedloop.Observer) = (*SyncObserverV2)(nil)
	_ (rangedloop.Partial)  = (*SyncObserverV2)(nil)
)

// NewSyncObserverV2 creates a new SyncObserverV2.
func NewSyncObserverV2(log *zap.Logger, config Config, overlay Overlay) *SyncObserverV2 {
	return &SyncObserverV2{
		log:     log,
		overlay: overlay,
		upload:  NewUpload(log, config),
		config:  config,
	}
}

// Start is called at the beginning of each segment loop.
func (observer *SyncObserverV2) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := observer.upload.CheckConfig(); err != nil {
		return err
	}

	observer.log.Debug("collecting bloom filters started")

	// load last piece counts from overlay db
	lastPieceCounts, err := observer.overlay.ActiveNodesPieceCounts(ctx)
	if err != nil {
		observer.log.Error("error getting last piece counts", zap.Error(err))
		err = nil
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int64)
	}

	observer.startTime = startTime
	observer.lastPieceCounts = lastPieceCounts
	observer.retainInfos = &concurrentRetainInfos{}
	observer.latestCreationTime = time.Time{}
	observer.seed = bloomfilter.GenerateSeed()
	return nil
}

// Fork returns itself as a partial.
func (observer *SyncObserverV2) Fork(context.Context) (rangedloop.Partial, error) {
	return observer, nil
}

// Join is a no-op.
func (*SyncObserverV2) Join(context.Context, rangedloop.Partial) error {
	return nil
}

// Finish uploads the bloom filters.
func (observer *SyncObserverV2) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := observer.upload.UploadBloomFilters(ctx, observer.latestCreationTime, observer.retainInfos); err != nil {
		return err
	}

	observer.log.Info("collecting bloom filters finished",
		zap.Uint64("inline segments", observer.inlineCount.Load()),
		zap.Uint64("remote segments", observer.remoteCount.Load()))

	return nil
}

// TestingRetainInfos returns retain infos collected by observer.
func (observer *SyncObserverV2) TestingRetainInfos() MinimalRetainInfoMap {
	return observer.retainInfos
}

// TestingForceTableSize sets a fixed size for tables. Used for testing.
func (observer *SyncObserverV2) TestingForceTableSize(size int) {
	observer.forcedTableSize = size
}

// Process adds pieces to the bloom filter from remote segments.
func (observer *SyncObserverV2) Process(ctx context.Context, segments []rangedloop.Segment) error {
	var latestCreationTime time.Time
	for _, segment := range segments {
		if segment.Inline() {
			observer.inlineCount.Add(1)
			continue
		}

		observer.remoteCount.Add(1)

		// This is a sanity check to detect if we're not running against
		// a live database.
		if segment.CreatedAt.After(observer.startTime) {
			observer.log.Error("segment created after loop started",
				zap.Stringer("StreamID", segment.StreamID),
				zap.Time("loop started", observer.startTime),
				zap.Time("segment created", segment.CreatedAt))
			return errs.New("segment created after loop started")
		}

		if latestCreationTime.Before(segment.CreatedAt) {
			latestCreationTime = segment.CreatedAt
		}

		deriver := segment.RootPieceID.Deriver()
		for _, piece := range segment.Pieces {
			pieceID := deriver.Derive(piece.StorageNode, int32(piece.Number))
			observer.add(piece.StorageNode, pieceID)
		}
	}

	observer.mu.Lock()
	if observer.latestCreationTime.Before(latestCreationTime) {
		observer.latestCreationTime = latestCreationTime
	}
	observer.mu.Unlock()

	return nil
}

// add adds a piece ID to the relevant node's RetainInfo.
func (observer *SyncObserverV2) add(nodeID storj.NodeID, pieceID storj.PieceID) {
	v, ok := observer.retainInfos.m.Load(nodeID)
	if !ok {
		v, _ = observer.retainInfos.m.LoadOrStore(nodeID, &concurrentRetainInfo{})
	}
	cri := v.(*concurrentRetainInfo)
	cri.mu.Lock()
	defer cri.mu.Unlock()

	if cri.info == nil {
		// If we know how many pieces a node should be storing, use that
		// number. Otherwise, use default.
		numPieces := observer.config.InitialPieces
		if pieceCounts, found := observer.lastPieceCounts[nodeID]; found {
			if pieceCounts > 0 {
				numPieces = pieceCounts
			}
		} else {
			// Node was not in lastPieceCounts, which means it was
			// disqualified, and we won't generate a bloom filter for
			// it.
			return
		}

		hashCount, tableSize := bloomfilter.OptimalParameters(numPieces, observer.config.FalsePositiveRate, observer.config.MaxBloomFilterSize)
		// Limit the size of the bloom filter to ensure we are under the
		// limit for RPC.
		if observer.forcedTableSize > 0 {
			tableSize = observer.forcedTableSize
		}
		filter := bloomfilter.NewExplicit(observer.seed, hashCount, tableSize)
		cri.info = &RetainInfo{
			Filter: filter,
		}
	}

	cri.info.Filter.Add(pieceID)
	cri.info.Count++
}
