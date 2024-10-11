// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/shared/nodeidmap"
)

var mon = monkit.Package()

// TestingObserver provides testing methods for bloom filter generation ranged loop observers.
type TestingObserver interface {
	TestingRetainInfos() nodeidmap.Map[*RetainInfo]
	TestingForceTableSize(size int)
}

// Overlay minimal set of overlay functions that are needed for the observer.
type Overlay interface {
	ActiveNodesPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int64, err error)
}

// RetainInfo contains info needed for a storage node to retain important data and delete garbage data.
type RetainInfo struct {
	Filter *bloomfilter.Filter
	Count  int
}

// Observer implements a rangedloop observer to collect bloom filters for the garbage collection.
//
// architecture: Observer
type Observer struct {
	log     *zap.Logger
	config  Config
	upload  *Upload
	overlay Overlay

	// The following fields are reset for each loop.
	startTime       time.Time
	lastPieceCounts map[storj.NodeID]int64
	retainInfos     nodeidmap.Map[*RetainInfo]
	creationTime    time.Time
	seed            byte

	forcedTableSize int
}

var _ (rangedloop.Observer) = (*Observer)(nil)
var _ (rangedloop.Partial) = (*observerFork)(nil)

// NewObserver creates a new instance of the gc rangedloop observer.
func NewObserver(log *zap.Logger, config Config, overlay Overlay) *Observer {
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
	obs.creationTime = time.Now()
	obs.seed = bloomfilter.GenerateSeed()
	return nil
}

// Fork creates a Partial to build bloom filters over a chunk of all the segments.
func (obs *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	return newObserverFork(obs.log.Named("gc observer"), obs.config, obs.lastPieceCounts, obs.seed, obs.startTime, obs.forcedTableSize), nil
}

// Join merges the bloom filters gathered by each Partial.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)
	pieceTracker, ok := partial.(*observerFork)
	if !ok {
		return errs.New("expected %T but got %T", pieceTracker, partial)
	}

	var failures []error

	// Update the count and merge the bloom filters for each node.
	obs.retainInfos.Add(pieceTracker.retainInfos,
		func(old *RetainInfo, new *RetainInfo) *RetainInfo {
			old.Count += new.Count
			if err := old.Filter.AddFilter(new.Filter); err != nil {
				failures = append(failures, err)
			}
			return old
		})

	if len(failures) > 0 {
		return errs.Combine(failures...)
	}

	// find oldest from all latest creation time and GC observer start
	for _, lct := range pieceTracker.latestCreationTime {
		if lct != (time.Time{}) && lct.Before(obs.creationTime) {
			obs.creationTime = lct
		}
	}

	return nil
}

// Finish uploads the bloom filters.
func (obs *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err := obs.upload.UploadBloomFilters(ctx, obs.creationTime, obs.retainInfos); err != nil {
		return err
	}
	obs.log.Debug("collecting bloom filters finished")
	return nil
}

// TestingRetainInfos returns retain infos collected by observer.
func (obs *Observer) TestingRetainInfos() nodeidmap.Map[*RetainInfo] {
	return obs.retainInfos
}

// TestingForceTableSize sets a fixed size for tables. Used for testing.
func (obs *Observer) TestingForceTableSize(size int) {
	obs.forcedTableSize = size
}

// TestingCreationTime gets the creation time which will be used to set bloom filter CreationDate.
func (obs *Observer) TestingCreationTime() time.Time {
	return obs.creationTime
}

type observerFork struct {
	log    *zap.Logger
	config Config
	// TODO: should we use int or int64 consistently for piece count (db type is int64)?
	pieceCounts map[storj.NodeID]int64
	seed        byte
	startTime   time.Time

	retainInfos nodeidmap.Map[*RetainInfo]
	// latestCreationTime will be used to set bloom filter CreationDate.
	// Because bloom filter service needs to be run against immutable database view
	// we can set CreationDate using this logic:
	// * find latest segment creation time for each source
	// * choose the oldest one from all latest creation time and GC observer start time
	latestCreationTime map[string]time.Time

	forcedTableSize int
}

// newObserverFork instantiates a new observer fork to process different segment range.
// The seed is passed so that it can be shared among all parallel forks.
func newObserverFork(log *zap.Logger, config Config, pieceCounts map[storj.NodeID]int64, seed byte, startTime time.Time, forcedTableSize int) *observerFork {
	return &observerFork{
		log:                log,
		config:             config,
		pieceCounts:        pieceCounts,
		seed:               seed,
		startTime:          startTime,
		forcedTableSize:    forcedTableSize,
		retainInfos:        nodeidmap.MakeSized[*RetainInfo](len(pieceCounts)),
		latestCreationTime: make(map[string]time.Time),
	}
}

// Process adds pieces to the bloom filter from remote segments.
func (fork *observerFork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	now := time.Now()
	for _, segment := range segments {
		if segment.Inline() {
			continue
		}

		if fork.config.ExcludeExpiredPieces && segment.Expired(now) {
			continue
		}

		fork.updateLatestCreationTime(segment)

		deriver := segment.RootPieceID.Deriver()
		for _, piece := range segment.Pieces {
			pieceID := deriver.Derive(piece.StorageNode, int32(piece.Number))
			fork.add(piece.StorageNode, pieceID)
		}
	}
	return nil
}

func (fork *observerFork) updateLatestCreationTime(segment rangedloop.Segment) {
	if lct, found := fork.latestCreationTime[segment.Source]; found {
		if lct.Before(segment.CreatedAt) {
			fork.latestCreationTime[segment.Source] = segment.CreatedAt
		}
	} else {
		fork.latestCreationTime[segment.Source] = segment.CreatedAt
	}
}

// add adds a pieceID to the relevant node's RetainInfo.
func (fork *observerFork) add(nodeID storj.NodeID, pieceID storj.PieceID) {
	info, ok := fork.retainInfos.Load(nodeID)
	if !ok {
		// If we know how many pieces a node should be storing, use that number. Otherwise use default.
		numPieces := fork.config.InitialPieces
		if pieceCounts, found := fork.pieceCounts[nodeID]; found {
			if pieceCounts > 0 {
				numPieces = pieceCounts
			}
		} else {
			// node was not in pieceCounts which means it was disqalified
			// and we won't generate bloom filter for it
			return
		}

		hashCount, tableSize := bloomfilter.OptimalParameters(numPieces, fork.config.FalsePositiveRate, fork.config.MaxBloomFilterSize)
		// limit size of bloom filter to ensure we are under the limit for RPC
		if fork.forcedTableSize > 0 {
			tableSize = fork.forcedTableSize
		}

		filter := bloomfilter.NewExplicit(fork.seed, hashCount, tableSize)
		info = &RetainInfo{
			Filter: filter,
		}
		fork.retainInfos.Store(nodeID, info)
	}

	info.Filter.Add(pieceID)
	info.Count++
}
