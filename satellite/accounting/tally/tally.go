// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metainfo"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("tally error")
	mon   = monkit.Package()
)

// Config contains configurable values for the tally service
type Config struct {
	Interval time.Duration `help:"how frequently the tally service should run" releaseDefault:"1h" devDefault:"30s"`
}

// Service is the tally service for data stored on each storage node
//
// architecture: Chore
type Service struct {
	log  *zap.Logger
	Loop sync2.Cycle

	metainfoLoop            *metainfo.Loop
	liveAccounting          accounting.Cache
	storagenodeAccountingDB accounting.StoragenodeAccounting
	projectAccountingDB     accounting.ProjectAccounting
}

// New creates a new tally Service
func New(log *zap.Logger, sdb accounting.StoragenodeAccounting, pdb accounting.ProjectAccounting, liveAccounting accounting.Cache, metainfoLoop *metainfo.Loop, interval time.Duration) *Service {
	return &Service{
		log:  log,
		Loop: *sync2.NewCycle(interval),

		metainfoLoop:            metainfoLoop,
		liveAccounting:          liveAccounting,
		storagenodeAccountingDB: sdb,
		projectAccountingDB:     pdb,
	}
}

// Run the tally service loop
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("Tally service starting up")

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.Tally(ctx)
		if err != nil {
			service.log.Error("tally failed", zap.Error(err))
		}
		return nil
	})
}

// Close stops the service and releases any resources.
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}

// Tally calculates data-at-rest usage once
func (service *Service) Tally(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// The live accounting store will only keep a delta to space used relative
	// to the latest tally. Since a new tally is beginning, we will zero it out
	// now. There is a window between this call and the point where the tally DB
	// transaction starts, during which some changes in space usage may be
	// double-counted (counted in the tally and also counted as a delta to
	// the tally). If that happens, it will be fixed at the time of the next
	// tally run.
	err = service.liveAccounting.ResetTotals(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// Fetch when the last tally happened so we can roughly calculate the byte-hours.
	lastTime, err := service.storagenodeAccountingDB.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return Error.Wrap(err)
	}
	if lastTime.IsZero() {
		lastTime = time.Now()
	}

	// add up all nodes and buckets
	observer := NewObserver(service.log.Named("observer"))
	err = service.metainfoLoop.Join(ctx, observer)
	if err != nil {
		return Error.Wrap(err)
	}
	finishTime := time.Now()

	// calculate byte hours, not just bytes
	hours := time.Since(lastTime).Hours()
	for id := range observer.Node {
		observer.Node[id] *= hours
	}

	// save the new results
	var errAtRest, errBucketInfo error
	if len(observer.Node) > 0 {
		err = service.storagenodeAccountingDB.SaveTallies(ctx, finishTime, observer.Node)
		if err != nil {
			errAtRest = errs.New("StorageNodeAccounting.SaveTallies failed: %v", err)
		}
	}

	if len(observer.Bucket) > 0 {
		err = service.projectAccountingDB.SaveTallies(ctx, finishTime, observer.Bucket)
		if err != nil {
			errAtRest = errs.New("ProjectAccounting.SaveTallies failed: %v", err)
		}
	}

	// report bucket metrics
	if len(observer.Bucket) > 0 {
		var total accounting.BucketTally
		for _, bucket := range observer.Bucket {
			monAccounting.IntVal("bucket.objects").Observe(bucket.ObjectCount)

			monAccounting.IntVal("bucket.segments").Observe(bucket.Segments())
			monAccounting.IntVal("bucket.inline_segments").Observe(bucket.InlineSegments)
			monAccounting.IntVal("bucket.remote_segments").Observe(bucket.RemoteSegments)

			monAccounting.IntVal("bucket.bytes").Observe(bucket.Bytes())
			monAccounting.IntVal("bucket.inline_bytes").Observe(bucket.InlineBytes)
			monAccounting.IntVal("bucket.remote_bytes").Observe(bucket.RemoteBytes)
			total.Combine(bucket)
		}
		monAccounting.IntVal("total.objects").Observe(total.ObjectCount) //locked

		monAccounting.IntVal("total.segments").Observe(total.Segments())            //locked
		monAccounting.IntVal("total.inline_segments").Observe(total.InlineSegments) //locked
		monAccounting.IntVal("total.remote_segments").Observe(total.RemoteSegments) //locked

		monAccounting.IntVal("total.bytes").Observe(total.Bytes())            //locked
		monAccounting.IntVal("total.inline_bytes").Observe(total.InlineBytes) //locked
		monAccounting.IntVal("total.remote_bytes").Observe(total.RemoteBytes) //locked
	}

	// return errors if something went wrong.
	return errs.Combine(errAtRest, errBucketInfo)
}

var _ metainfo.Observer = (*Observer)(nil)

// Observer observes metainfo and adds up tallies for nodes and buckets
type Observer struct {
	Log    *zap.Logger
	Node   map[storj.NodeID]float64
	Bucket map[string]*accounting.BucketTally
}

// NewObserver returns an metainfo loop observer that adds up totals for buckets and nodes.
func NewObserver(log *zap.Logger) *Observer {
	return &Observer{
		Log:    log,
		Node:   make(map[storj.NodeID]float64),
		Bucket: make(map[string]*accounting.BucketTally),
	}
}

// ensureBucket returns bucket corresponding to the passed in path
func (observer *Observer) ensureBucket(ctx context.Context, path metainfo.ScopedPath) *accounting.BucketTally {
	bucketID := storj.JoinPaths(path.ProjectIDString, path.BucketName)

	bucket, exists := observer.Bucket[bucketID]
	if !exists {
		bucket = &accounting.BucketTally{}
		bucket.ProjectID = path.ProjectID
		bucket.BucketName = []byte(path.BucketName)
		observer.Bucket[bucketID] = bucket
	}

	return bucket
}

// Object is called for each object once.
func (observer *Observer) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	bucket := observer.ensureBucket(ctx, path)
	bucket.ObjectCount++
	return nil
}

// InlineSegment is called for each inline segment.
func (observer *Observer) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	bucket := observer.ensureBucket(ctx, path)
	bucket.InlineSegments++
	bucket.InlineBytes += int64(len(pointer.InlineSegment))
	bucket.MetadataSize += int64(len(pointer.Metadata))

	return nil
}

// RemoteSegment is called for each remote segment.
func (observer *Observer) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) (err error) {
	bucket := observer.ensureBucket(ctx, path)
	bucket.RemoteSegments++
	bucket.RemoteBytes += pointer.GetSegmentSize()
	bucket.MetadataSize += int64(len(pointer.Metadata))

	// add node info
	remote := pointer.GetRemote()
	redundancy := remote.GetRedundancy()
	segmentSize := pointer.GetSegmentSize()
	minimumRequired := redundancy.GetMinReq()

	if remote == nil || redundancy == nil || minimumRequired <= 0 {
		observer.Log.Error("failed sanity check", zap.String("path", path.Raw))
		return nil
	}

	pieceSize := float64(segmentSize / int64(minimumRequired))
	for _, piece := range remote.GetRemotePieces() {
		observer.Node[piece.NodeId] += pieceSize
	}
	return nil
}

// using custom name to avoid breaking monitoring
var monAccounting = monkit.ScopeNamed("storj.io/storj/satellite/accounting")
