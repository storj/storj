// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/bloomfilter"
)

var mon = monkit.Package()

// Overlay minimal set of overlay functions that are needed for the observer.
type Overlay interface {
	ActiveNodesPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int64, err error)
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data.
type RetainInfo struct {
	Filter *bloomfilter.Filter
	Count  int
}

// MinimalRetainInfoMap is what is exposed by the observer to the upload.
type MinimalRetainInfoMap interface {
	IsEmpty() bool
	Load(nodeID storj.NodeID) (info *RetainInfo, ok bool)
	Range(f func(nodeID storj.NodeID, info *RetainInfo) bool)
}

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

// Observer collects bloom filters for the garbage collection.
//
// architecture: Observer
type Observer struct {
	log     *zap.Logger
	config  Config
	overlay Overlay
	upload  *Upload

	retainInfos     *concurrentRetainInfos
	forcedTableSize int

	// publishGuard, when set, has to pass before a finished generation is
	// published.
	publishGuard func(ctx context.Context) error

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
	_ (rangedloop.Observer) = (*Observer)(nil)
	_ (rangedloop.Partial)  = (*Observer)(nil)
)

// NewObserver creates a new Observer.
func NewObserver(log *zap.Logger, config Config, overlay Overlay) *Observer {
	return &Observer{
		log:     log,
		overlay: overlay,
		upload:  NewUpload(log, config),
		config:  config,
	}
}

// SetPublishGuard registers a check that has to pass before a finished
// generation is published. It runs at the end of every pass, once the scan is
// complete but before anything is uploaded, so a scan whose counts do not add
// up leaves no generation behind: nodes would delete the live pieces such a
// filter missed, and rerunning is cheaper than that.
func (observer *Observer) SetPublishGuard(guard func(ctx context.Context) error) {
	observer.publishGuard = guard
}

// ProcessedSegments returns the number of segments the current pass has seen.
func (observer *Observer) ProcessedSegments() uint64 {
	return observer.inlineCount.Load() + observer.remoteCount.Load()
}

// Start is called at the beginning of each segment loop.
func (observer *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
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
	observer.inlineCount.Store(0)
	observer.remoteCount.Store(0)
	return nil
}

// Fork returns itself as a partial.
func (observer *Observer) Fork(context.Context) (rangedloop.Partial, error) {
	return observer, nil
}

// Join is a no-op.
func (*Observer) Join(context.Context, rangedloop.Partial) error {
	return nil
}

// Finish uploads the bloom filters.
func (observer *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if observer.publishGuard != nil {
		if err := observer.publishGuard(ctx); err != nil {
			return err
		}
	}

	if err := observer.upload.UploadBloomFilters(ctx, observer.latestCreationTime, observer.retainInfos); err != nil {
		return err
	}

	observer.log.Info("collecting bloom filters finished",
		zap.Uint64("inline_segments", observer.inlineCount.Load()),
		zap.Uint64("remote_segments", observer.remoteCount.Load()))

	return nil
}

// TestingRetainInfos returns retain infos collected by observer.
func (observer *Observer) TestingRetainInfos() MinimalRetainInfoMap {
	return observer.retainInfos
}

// TestingForceTableSize sets a fixed size for tables. Used for testing.
func (observer *Observer) TestingForceTableSize(size int) {
	observer.forcedTableSize = size
}

// Process adds pieces to the bloom filter from remote segments.
func (observer *Observer) Process(ctx context.Context, segments []rangedloop.Segment) error {
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
				zap.Stringer("stream_id", segment.StreamID),
				zap.Time("loop_started", observer.startTime),
				zap.Time("segment_created", segment.CreatedAt))
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
func (observer *Observer) add(nodeID storj.NodeID, pieceID storj.PieceID) {
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
