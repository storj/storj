// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/time/rate"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

var (
	// LoopError is a standard error class for this component.
	LoopError = errs.Class("metainfo loop error")
	// LoopClosedError is a loop closed error.
	LoopClosedError = LoopError.New("loop closed")
)

// Object is the object info passed to Observer by metainfo loop.
type Object struct {
	Location       metabase.ObjectLocation // tally
	SegmentCount   int                     // metrics
	LastSegment    *Segment                // metrics
	expirationDate time.Time               // tally
}

// Expired checks if object is expired relative to now.
func (object *Object) Expired(now time.Time) bool {
	return !object.expirationDate.IsZero() && object.expirationDate.Before(now)
}

// Segment is the segment info passed to Observer by metainfo loop.
type Segment struct {
	Location       metabase.SegmentLocation // tally, repair, graceful exit, audit
	StreamID       uuid.UUID                // audit
	DataSize       int                      // tally, graceful exit
	MetadataSize   int                      // tally
	Inline         bool                     // metrics
	Redundancy     storj.RedundancyScheme   // tally, graceful exit, repair
	RootPieceID    storj.PieceID            // gc, graceful exit
	Pieces         metabase.Pieces          // tally, audit, gc, graceful exit, repair
	CreationDate   time.Time                // repair
	expirationDate time.Time                // tally, repair
	LastRepaired   time.Time                // repair
	Pointer        *pb.Pointer              // repair
}

// Expired checks if segment is expired relative to now.
func (segment *Segment) Expired(now time.Time) bool {
	return !segment.expirationDate.IsZero() && segment.expirationDate.Before(now)
}

// Observer is an interface defining an observer that can subscribe to the metainfo loop.
//
// architecture: Observer
type Observer interface {
	Object(context.Context, *Object) error
	RemoteSegment(context.Context, *Segment) error
	InlineSegment(context.Context, *Segment) error
}

// NullObserver is an observer that does nothing. This is useful for joining
// and ensuring the metainfo loop runs once before you use a real observer.
type NullObserver struct{}

// Object implements the Observer interface.
func (NullObserver) Object(context.Context, *Object) error {
	return nil
}

// RemoteSegment implements the Observer interface.
func (NullObserver) RemoteSegment(context.Context, *Segment) error {
	return nil
}

// InlineSegment implements the Observer interface.
func (NullObserver) InlineSegment(context.Context, *Segment) error {
	return nil
}

type observerContext struct {
	observer Observer

	ctx  context.Context
	done chan error

	object *monkit.DurationDist
	remote *monkit.DurationDist
	inline *monkit.DurationDist
}

func newObserverContext(ctx context.Context, obs Observer) *observerContext {
	name := fmt.Sprintf("%T", obs)
	key := monkit.NewSeriesKey("observer").WithTag("name", name)

	return &observerContext{
		observer: obs,

		ctx:  ctx,
		done: make(chan error),

		object: monkit.NewDurationDist(key.WithTag("pointer_type", "object")),
		inline: monkit.NewDurationDist(key.WithTag("pointer_type", "inline")),
		remote: monkit.NewDurationDist(key.WithTag("pointer_type", "remote")),
	}
}

func (observer *observerContext) Object(ctx context.Context, object *Object) error {
	start := time.Now()
	defer func() { observer.object.Insert(time.Since(start)) }()

	return observer.observer.Object(ctx, object)
}

func (observer *observerContext) RemoteSegment(ctx context.Context, segment *Segment) error {
	start := time.Now()
	defer func() { observer.remote.Insert(time.Since(start)) }()

	return observer.observer.RemoteSegment(ctx, segment)
}

func (observer *observerContext) InlineSegment(ctx context.Context, segment *Segment) error {
	start := time.Now()
	defer func() { observer.inline.Insert(time.Since(start)) }()

	return observer.observer.InlineSegment(ctx, segment)
}

func (observer *observerContext) HandleError(err error) bool {
	if err != nil {
		observer.done <- err
		observer.Finish()
		return true
	}
	return false
}

func (observer *observerContext) Finish() {
	close(observer.done)

	name := fmt.Sprintf("%T", observer.observer)
	stats := allObserverStatsCollectors.GetStats(name)
	stats.Observe(observer)
}

func (observer *observerContext) Wait() error {
	return <-observer.done
}

type observers []*observerContext

func (o *observers) Remove(toRemove *observerContext) {
	list := *o
	for i, observer := range list {
		if observer == toRemove {
			list[len(list)-1], list[i] = list[i], list[len(list)-1]
			*o = list[:len(list)-1]
			return
		}
	}
}

func (o *observers) Finish() {
	for _, observer := range *o {
		observer.Finish()
	}
}

// LoopConfig contains configurable values for the metainfo loop.
type LoopConfig struct {
	CoalesceDuration time.Duration `help:"how long to wait for new observers before starting iteration" releaseDefault:"5s" devDefault:"5s"`
	RateLimit        float64       `help:"rate limit (default is 0 which is unlimited segments per second)" default:"0"`
	ListLimit        int           `help:"how many items to query in a batch" default:"2500"`
}

// Loop is a metainfo loop service.
//
// architecture: Service
type Loop struct {
	config     LoopConfig
	db         PointerDB
	bucketsDB  BucketsDB
	metabaseDB MetabaseDB
	join       chan []*observerContext
	done       chan struct{}
}

// NewLoop creates a new metainfo loop service.
func NewLoop(config LoopConfig, db PointerDB, bucketsDB BucketsDB, metabaseDB MetabaseDB) *Loop {
	return &Loop{
		db:         db,
		bucketsDB:  bucketsDB,
		metabaseDB: metabaseDB,
		config:     config,
		join:       make(chan []*observerContext),
		done:       make(chan struct{}),
	}
}

// Join will join the looper for one full cycle until completion and then returns.
// On ctx cancel the observer will return without completely finishing.
// Only on full complete iteration it will return nil.
// Safe to be called concurrently.
func (loop *Loop) Join(ctx context.Context, observers ...Observer) (err error) {
	defer mon.Task()(&ctx)(&err)

	obsContexts := make([]*observerContext, len(observers))
	for i, obs := range observers {
		obsContexts[i] = newObserverContext(ctx, obs)
	}

	select {
	case loop.join <- obsContexts:
	case <-ctx.Done():
		return ctx.Err()
	case <-loop.done:
		return LoopClosedError
	}

	var errList errs.Group
	for _, ctx := range obsContexts {
		errList.Add(ctx.Wait())
	}

	return errList.Err()
}

// Run starts the looping service.
// It can only be called once, otherwise a panic will occur.
func (loop *Loop) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err := loop.runOnce(ctx)
		if err != nil {
			return err
		}
	}
}

// Close closes the looping services.
func (loop *Loop) Close() (err error) {
	close(loop.done)
	return nil
}

// runOnce goes through metainfo one time and sends information to observers.
func (loop *Loop) runOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var observers []*observerContext

	// wait for the first observer, or exit because context is canceled
	select {
	case list := <-loop.join:
		observers = append(observers, list...)
	case <-ctx.Done():
		return ctx.Err()
	}

	// after the first observer is found, set timer for CoalesceDuration and add any observers that try to join before the timer is up
	timer := time.NewTimer(loop.config.CoalesceDuration)
waitformore:
	for {
		select {
		case list := <-loop.join:
			observers = append(observers, list...)
		case <-timer.C:
			break waitformore
		case <-ctx.Done():
			finishObservers(observers)
			return ctx.Err()
		}
	}
	return iterateDatabase(ctx, loop.db, loop.bucketsDB, loop.metabaseDB, observers, loop.config.ListLimit, rate.NewLimiter(rate.Limit(loop.config.RateLimit), 1))
}

// IterateDatabase iterates over PointerDB and notifies specified observers about results.
//
// It uses 10000 as the lookup limit for iterating.
func IterateDatabase(ctx context.Context, rateLimit float64, db PointerDB, bucketsDB BucketsDB, metabaseDB MetabaseDB, observers ...Observer) error {
	obsContexts := make([]*observerContext, len(observers))
	for i, observer := range observers {
		obsContexts[i] = newObserverContext(ctx, observer)
	}
	return iterateDatabase(ctx, db, bucketsDB, metabaseDB, obsContexts, 10000, rate.NewLimiter(rate.Limit(rateLimit), 1))
}

// Wait waits for run to be finished.
// Safe to be called concurrently.
func (loop *Loop) Wait() {
	<-loop.done
}

func iterateDatabase(ctx context.Context, db PointerDB, bucketsDB BucketsDB, metabaseDB MetabaseDB, observers observers, limit int, rateLimiter *rate.Limiter) (err error) {
	defer func() {
		if err != nil {
			for _, observer := range observers {
				observer.HandleError(err)
			}
			return
		}
		observers.Finish()
	}()

	more := true
	bucketsCursor := ListAllBucketsCursor{}
	for more {
		buckets, err := bucketsDB.ListAllBuckets(ctx, ListAllBucketsOptions{
			Cursor: bucketsCursor,
			Limit:  limit,
		})
		if err != nil {
			return LoopError.Wrap(err)
		}

		for _, bucket := range buckets.Items {
			err := iterateObjects(ctx, bucket.ProjectID, bucket.Name, metabaseDB, observers, limit, rateLimiter)
			if err != nil {
				return LoopError.Wrap(err)
			}
		}

		more = buckets.More
		if more {
			lastBucket := buckets.Items[len(buckets.Items)-1]
			bucketsCursor.ProjectID = lastBucket.ProjectID
			bucketsCursor.BucketName = []byte(lastBucket.Name)
		}
	}
	return err
}

func iterateObjects(ctx context.Context, projectID uuid.UUID, bucket string, metabaseDB MetabaseDB, observers observers, limit int, rateLimiter *rate.Limiter) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO we should improve performance here, this is just most straightforward solution

	err = metabaseDB.IterateObjectsAllVersions(ctx, metabase.IterateObjects{
		ProjectID:  projectID,
		BucketName: bucket,
		BatchSize:  limit,
		Recursive:  true,
		Status:     metabase.Committed, // TODO we should iterate also Pending objects
	}, func(ctx context.Context, it metabase.ObjectsIterator) error {
		var entry metabase.ObjectEntry
		for it.Next(ctx, &entry) {
			if err := rateLimiter.Wait(ctx); err != nil {
				// We don't really execute concurrent batches so we should never
				// exceed the burst size of 1 and this should never happen.
				// We can also enter here if the context is cancelled.
				return err
			}

			for _, observer := range observers {
				location := metabase.ObjectLocation{
					ProjectID:  projectID,
					BucketName: bucket,
					ObjectKey:  entry.ObjectKey,
				}
				keepObserver := handleObject(ctx, observer, location, entry)
				if !keepObserver {
					observers.Remove(observer)
				}
			}

			if len(observers) == 0 {
				return nil
			}

			// if context has been canceled exit. Otherwise, continue
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			err = iterateSegments(ctx, entry.StreamID, projectID, bucket, entry.ObjectKey, metabaseDB, observers, limit, rateLimiter)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func iterateSegments(ctx context.Context, streamID uuid.UUID, projectID uuid.UUID, bucket string, objectKey metabase.ObjectKey, metabaseDB MetabaseDB, observers observers, limit int, rateLimiter *rate.Limiter) (err error) {
	defer mon.Task()(&ctx)(&err)

	more := true
	cursor := metabase.SegmentPosition{}
	for more {
		if err := rateLimiter.Wait(ctx); err != nil {
			// We don't really execute concurrent batches so we should never
			// exceed the burst size of 1 and this should never happen.
			// We can also enter here if the context is cancelled.
			return err
		}

		segments, err := metabaseDB.ListSegments(ctx, metabase.ListSegments{
			StreamID: streamID,
			Cursor:   cursor,
			Limit:    limit,
		})
		if err != nil {
			return err
		}

		for _, segment := range segments.Segments {
			for _, observer := range observers {
				location := metabase.SegmentLocation{
					ProjectID:  projectID,
					BucketName: bucket,
					ObjectKey:  objectKey,
					Position:   segment.Position,
				}
				keepObserver := handleSegment(ctx, observer, location, segment)
				if !keepObserver {
					observers.Remove(observer)
				}
			}

			if len(observers) == 0 {
				return nil
			}

			// if context has been canceled exit. Otherwise, continue
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		more = segments.More
		if more {
			lastSegment := segments.Segments[len(segments.Segments)-1]
			cursor = lastSegment.Position
		}
	}

	return nil
}

func handleObject(ctx context.Context, observer *observerContext, location metabase.ObjectLocation, object metabase.ObjectEntry) bool {
	expirationDate := time.Time{}
	if object.ExpiresAt != nil {
		expirationDate = *object.ExpiresAt
	}

	if observer.HandleError(observer.Object(ctx, &Object{
		Location:       location,
		SegmentCount:   int(object.SegmentCount),
		expirationDate: expirationDate,
		LastSegment:    &Segment{}, // TODO ideally would be to remove this field
	})) {
		return false
	}

	select {
	case <-observer.ctx.Done():
		observer.HandleError(observer.ctx.Err())
		return false
	default:
	}

	return true
}

func handleSegment(ctx context.Context, observer *observerContext, location metabase.SegmentLocation, segment metabase.Segment) bool {
	loopSegment := &Segment{
		Location: location,
	}

	loopSegment.StreamID = segment.StreamID
	loopSegment.DataSize = int(segment.EncryptedSize) // TODO should this be plain or enrypted size
	if segment.Inline() {
		loopSegment.Inline = true
		if observer.HandleError(observer.InlineSegment(ctx, loopSegment)) {
			return false
		}
	} else {
		loopSegment.RootPieceID = segment.RootPieceID
		loopSegment.Redundancy = segment.Redundancy
		loopSegment.Pieces = segment.Pieces
		if observer.HandleError(observer.RemoteSegment(ctx, loopSegment)) {
			return false
		}
	}

	select {
	case <-observer.ctx.Done():
		observer.HandleError(observer.ctx.Err())
		return false
	default:
	}

	return true
}

func finishObservers(observers []*observerContext) {
	for _, observer := range observers {
		observer.Finish()
	}
}
