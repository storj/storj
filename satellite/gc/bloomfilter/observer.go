// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"encoding/binary"
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
			// Should not happen: add only stores entries for nodes it
			// also initializes a filter for. Skip defensively.
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

	// shard is the node shard being processed; it advances on each loop
	// unless pinned by config.Shard. uploadPrefix is shared by all passes.
	shard         int
	uploadPrefix  string
	freshPrefix   bool
	finishErr     error
	runningPasses bool

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
	if config.ShardCount < 1 {
		config.ShardCount = 1
	}
	if config.ShardCount == 1 {
		// there is only one shard to run, so pinning it is a no-op; this also
		// keeps the zero value of Shard from being read as "pin shard 0".
		config.Shard = -1
	}
	return &Observer{
		log:     log,
		overlay: overlay,
		upload:  NewUpload(log, config),
		config:  config,
		shard:   -1,
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

// shardOf returns the shard for a node ID: shards are contiguous prefix
// ranges of the node ID space, so they are stable regardless of node count.
func (observer *Observer) shardOf(id storj.NodeID) int {
	return int(uint64(binary.BigEndian.Uint32(id[:4])) * uint64(observer.config.ShardCount) >> 32)
}

// Start is called at the beginning of each segment loop.
func (observer *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// The ranged loop only logs errors from Start, Fork, Process, Join and
	// Finish, and skips Finish entirely when an earlier step failed. Assume
	// failure until Finish says otherwise, so the run-once caller does not
	// mistake a skipped shard for a completed one.
	observer.finishErr = errs.New("bloom filter generation did not finish")

	if observer.config.ShardCount > 1 && !observer.runningPasses {
		// The shard cursor and the upload prefix only live in this process, so
		// a rotation spread over restarts would keep publishing generations
		// that cover shard 0 only.
		return errs.New("ShardCount %d requires the run-once job, which runs every shard in one process", observer.config.ShardCount)
	}

	if err := observer.upload.CheckConfig(); err != nil {
		return err
	}

	observer.log.Debug("collecting bloom filters started")

	// load last piece counts from overlay db; a missing or partial node list
	// would silently produce a generation without those nodes' filters
	lastPieceCounts, err := observer.overlay.ActiveNodesPieceCounts(ctx)
	if err != nil {
		observer.log.Error("error getting last piece counts", zap.Error(err))
		return err
	}

	if observer.config.Shard >= 0 {
		observer.shard = observer.config.Shard
	} else {
		observer.shard = (observer.shard + 1) % observer.config.ShardCount
	}
	if observer.config.ShardCount > 1 {
		// Copy, because Overlay does not hand over ownership of the map and
		// the next pass needs the full set of nodes again.
		shardPieceCounts := make(map[storj.NodeID]int64, len(lastPieceCounts)/observer.config.ShardCount+1)
		for id, count := range lastPieceCounts {
			if observer.shardOf(id) == observer.shard {
				shardPieceCounts[id] = count
			}
		}
		lastPieceCounts = shardPieceCounts
	}
	observer.freshPrefix = false
	if observer.uploadPrefix == "" {
		observer.uploadPrefix = observer.config.UploadPrefix
		if observer.uploadPrefix == "" {
			observer.uploadPrefix = time.Now().Format(time.RFC3339Nano)
			observer.freshPrefix = true
		}
	}
	observer.log.Info("processing node shard",
		zap.Int("shard", observer.shard),
		zap.Int("shard_count", observer.config.ShardCount),
		zap.Int("nodes", len(lastPieceCounts)),
		zap.String("upload_prefix", observer.uploadPrefix))

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
	defer func() { observer.finishErr = err }()

	if observer.publishGuard != nil {
		if err := observer.publishGuard(ctx); err != nil {
			return err
		}
	}

	if err := observer.upload.UploadBloomFilters(ctx, observer.latestCreationTime, observer.retainInfos, observer.uploadPrefix, observer.shard, observer.freshPrefix); err != nil {
		return err
	}

	uploadPrefix := observer.uploadPrefix

	// Rotate the shared prefix after the last shard, so the next cycle
	// uploads into a fresh generation instead of overwriting this one.
	if observer.config.UploadPrefix == "" && observer.config.Shard < 0 && observer.shard == observer.config.ShardCount-1 {
		observer.uploadPrefix = ""
	}

	observer.log.Info("collecting bloom filters finished",
		zap.Int("shard", observer.shard),
		zap.String("upload_prefix", uploadPrefix),
		zap.Uint64("inline_segments", observer.inlineCount.Load()),
		zap.Uint64("remote_segments", observer.remoteCount.Load()))

	return nil
}

// FinishError returns the error from the last loop iteration, which the
// ranged loop only logs. It is also non-nil when the loop failed before
// reaching Finish.
func (observer *Observer) FinishError() error { return observer.finishErr }

// RunPasses runs runOnce once per shard, stopping at the first shard that did
// not finish. All shards share one upload prefix, so a cycle that skips a
// shard would publish a generation covering only some of the nodes.
func (observer *Observer) RunPasses(ctx context.Context, runOnce func(ctx context.Context) error) error {
	observer.runningPasses = true
	defer func() { observer.runningPasses = false }()

	passes := observer.config.ShardCount
	if observer.config.Shard >= 0 {
		passes = 1
	}
	for range passes {
		// Start arms this too, but it is not guaranteed to be reached: an
		// observer missing from the loop would otherwise look successful.
		observer.finishErr = errs.New("bloom filter generation did not run")
		if err := runOnce(ctx); err != nil {
			return err
		}
		// the ranged loop only logs observer errors, so a shard that failed
		// or was skipped must not pass silently.
		if err := observer.FinishError(); err != nil {
			return err
		}
	}
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
			if _, ok := observer.lastPieceCounts[piece.StorageNode]; !ok {
				// disqualified or outside the current shard
				continue
			}
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
		// number. Otherwise, use default. Process only calls add for
		// nodes present in lastPieceCounts.
		numPieces := observer.config.InitialPieces
		if pieceCounts := observer.lastPieceCounts[nodeID]; pieceCounts > 0 {
			numPieces = pieceCounts
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
