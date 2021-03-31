// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metaloop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/time/rate"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

const batchsizeLimit = 2500

var (
	mon = monkit.Package()

	// Error is a standard error class for this component.
	Error = errs.Class("metainfo loop error")
	// ErrClosed is a loop closed error.
	ErrClosed = Error.New("loop closed")
)

// Object is the object info passed to Observer by metainfo loop.
type Object metabase.LoopObjectEntry

// Expired checks if object expired relative to now.
func (object *Object) Expired(now time.Time) bool {
	return object.ExpiresAt != nil && object.ExpiresAt.Before(now)
}

// Segment is the segment info passed to Observer by metainfo loop.
type Segment struct {
	Location       metabase.SegmentLocation // tally, repair, graceful exit, audit
	CreationDate   time.Time                // repair
	ExpirationDate time.Time                // tally, repair
	LastRepaired   time.Time                // repair

	metabase.LoopSegmentEntry
}

// Expired checks if segment is expired relative to now.
func (segment *Segment) Expired(now time.Time) bool {
	return !segment.ExpirationDate.IsZero() && segment.ExpirationDate.Before(now)
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

// Config contains configurable values for the metainfo loop.
type Config struct {
	CoalesceDuration time.Duration `help:"how long to wait for new observers before starting iteration" releaseDefault:"5s" devDefault:"5s"`
	RateLimit        float64       `help:"rate limit (default is 0 which is unlimited segments per second)" default:"0"`
	ListLimit        int           `help:"how many items to query in a batch" default:"2500"`
}

// MetabaseDB contains iterators for the metabase data.
type MetabaseDB interface {
	// IterateLoopObjects iterates through all objects in metabase for metainfo loop purpose.
	IterateLoopObjects(ctx context.Context, opts metabase.IterateLoopObjects, fn func(context.Context, metabase.LoopObjectsIterator) error) (err error)
	// IterateLoopStreams iterates through all streams passed in as arguments.
	IterateLoopStreams(ctx context.Context, opts metabase.IterateLoopStreams, handleStream func(ctx context.Context, streamID uuid.UUID, next metabase.SegmentIterator) error) (err error)
}

// Service is a metainfo loop service.
//
// architecture: Service
type Service struct {
	config     Config
	metabaseDB MetabaseDB
	join       chan []*observerContext
	done       chan struct{}
}

// New creates a new metainfo loop service.
func New(config Config, metabaseDB MetabaseDB) *Service {
	return &Service{
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
func (loop *Service) Join(ctx context.Context, observers ...Observer) (err error) {
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
		return ErrClosed
	}

	var errList errs.Group
	for _, ctx := range obsContexts {
		errList.Add(ctx.Wait())
	}

	return errList.Err()
}

// Run starts the looping service.
// It can only be called once, otherwise a panic will occur.
func (loop *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err := loop.RunOnce(ctx)
		if err != nil {
			return err
		}
	}
}

// Close closes the looping services.
func (loop *Service) Close() (err error) {
	close(loop.done)
	return nil
}

// RunOnce goes through metainfo one time and sends information to observers.
//
// It is not safe to call this concurrently with Run.
func (loop *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err) //mon:locked

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
	return iterateDatabase(ctx, loop.metabaseDB, observers, loop.config.ListLimit, rate.NewLimiter(rate.Limit(loop.config.RateLimit), 1))
}

// Wait waits for run to be finished.
// Safe to be called concurrently.
func (loop *Service) Wait() {
	<-loop.done
}

func iterateDatabase(ctx context.Context, metabaseDB MetabaseDB, observers []*observerContext, limit int, rateLimiter *rate.Limiter) (err error) {
	defer func() {
		if err != nil {
			for _, observer := range observers {
				observer.HandleError(err)
			}
			return
		}
		finishObservers(observers)
	}()

	observers, err = iterateObjects(ctx, metabaseDB, observers, limit, rateLimiter)
	if err != nil {
		return Error.Wrap(err)
	}

	return err
}

func iterateObjects(ctx context.Context, metabaseDB MetabaseDB, observers []*observerContext, limit int, rateLimiter *rate.Limiter) (_ []*observerContext, err error) {
	defer mon.Task()(&ctx)(&err)

	if limit <= 0 || limit > batchsizeLimit {
		limit = batchsizeLimit
	}

	startingTime := time.Now()

	noObserversErr := errs.New("no observers")

	// TODO we may consider keeping only expiration time as its
	// only thing we need to handle segments
	objectsMap := make(map[uuid.UUID]metabase.LoopObjectEntry)
	ids := make([]uuid.UUID, 0, limit)

	var objectsProcessed, segmentsProcessed int64

	processBatch := func(ctx context.Context) (err error) {
		defer mon.TaskNamed("processBatch")(&ctx)(&err)

		if len(objectsMap) == 0 {
			return nil
		}

		err = metabaseDB.IterateLoopStreams(ctx, metabase.IterateLoopStreams{
			StreamIDs:      ids,
			AsOfSystemTime: startingTime,
		}, func(ctx context.Context, streamID uuid.UUID, next metabase.SegmentIterator) (err error) {
			defer mon.TaskNamed("iterateLoopStreamsCB")(&ctx, "objs", objectsProcessed, "segs", segmentsProcessed)(&err)

			if err := ctx.Err(); err != nil {
				return err
			}

			obj, ok := objectsMap[streamID]
			if !ok {
				return Error.New("unable to find corresponding object: %v", streamID)
			}
			delete(objectsMap, streamID)

			observers = withObservers(ctx, observers, func(ctx context.Context, observer *observerContext) bool {
				object := Object(obj)
				return !observer.HandleError(handleObject(ctx, observer, &object))
			})
			if len(observers) == 0 {
				return noObserversErr
			}

			objectsProcessed++
			mon.IntVal("objectsProcessed").Observe(objectsProcessed) //mon:locked

			for {
				// if context has been canceled exit. Otherwise, continue
				if err := ctx.Err(); err != nil {
					return err
				}

				var segment metabase.LoopSegmentEntry
				if !next(&segment) {
					break
				}

				location := metabase.SegmentLocation{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  obj.ObjectKey,
					Position:   segment.Position,
				}

				observers = withObservers(ctx, observers, func(ctx context.Context, observer *observerContext) bool {
					return !observer.HandleError(handleSegment(ctx, observer, location, segment, obj.ExpiresAt))
				})
				if len(observers) == 0 {
					return noObserversErr
				}

				segmentsProcessed++
				mon.IntVal("segmentsProcessed").Observe(segmentsProcessed) //mon:locked

			}

			return nil
		})
		if err != nil {
			return Error.Wrap(err)
		}

		if len(objectsMap) > 0 {
			return Error.New("unhandled objects %#v", objectsMap)
		}

		return nil
	}

	var objectsIterated int64

	segmentsInBatch := int32(0)
	err = metabaseDB.IterateLoopObjects(ctx, metabase.IterateLoopObjects{
		BatchSize:      limit,
		AsOfSystemTime: startingTime,
	}, func(ctx context.Context, it metabase.LoopObjectsIterator) (err error) {
		defer mon.TaskNamed("iterateLoopObjectsCB")(&ctx)(&err)
		var entry metabase.LoopObjectEntry
		for it.Next(ctx, &entry) {
			timer := mon.Timer("iterateLoopObjectsRateLimit").Start()
			if err := rateLimiter.Wait(ctx); err != nil {
				// We don't really execute concurrent batches so we should never
				// exceed the burst size of 1 and this should never happen.
				// We can also enter here if the context is cancelled.
				timer.Stop()
				return err
			}
			timer.Stop()

			mon.IntVal("objectsIterated").Observe(objectsIterated) //mon:locked
			objectsProcessed++

			objectsMap[entry.StreamID] = entry
			ids = append(ids, entry.StreamID)

			// add +1 to reduce risk of crossing limit
			segmentsInBatch += entry.SegmentCount + 1

			if segmentsInBatch >= int32(limit) {
				err := processBatch(ctx)
				if err != nil {
					if errors.Is(err, noObserversErr) {
						return nil
					}
					return err
				}

				if len(objectsMap) > 0 {
					return errs.New("objects map is not empty")
				}

				ids = ids[:0]
				segmentsInBatch = 0
			}
		}
		err = processBatch(ctx)
		if errors.Is(err, noObserversErr) {
			return nil
		}
		return err
	})

	return observers, err
}

func withObservers(ctx context.Context, observers []*observerContext, handleObserver func(ctx context.Context, observer *observerContext) bool) []*observerContext {
	defer mon.Task()(&ctx)(nil)
	nextObservers := observers[:0]
	for _, observer := range observers {
		keepObserver := handleObserver(ctx, observer)
		if keepObserver {
			nextObservers = append(nextObservers, observer)
		}
	}
	return nextObservers
}

func handleObject(ctx context.Context, observer *observerContext, object *Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := observer.Object(ctx, object); err != nil {
		return err
	}

	return observer.ctx.Err()
}

func handleSegment(ctx context.Context, observer *observerContext, location metabase.SegmentLocation, segment metabase.LoopSegmentEntry, expirationDate *time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	loopSegment := &Segment{
		Location: location,
		// TODO we are not setting this since multipart-upload branch, we need to
		// check if thats affecting anything and if we need to set it correctly
		// or just replace it with something else
		CreationDate: time.Time{},
		// TODO we are not setting this and we need to decide what to do with this
		LastRepaired: time.Time{},

		LoopSegmentEntry: segment,
	}

	if expirationDate != nil {
		loopSegment.ExpirationDate = *expirationDate
	}

	if loopSegment.Inline() {
		if err := observer.InlineSegment(ctx, loopSegment); err != nil {
			return err
		}
	} else {
		if err := observer.RemoteSegment(ctx, loopSegment); err != nil {
			return err
		}
	}

	return observer.ctx.Err()
}

func finishObservers(observers []*observerContext) {
	for _, observer := range observers {
		observer.Finish()
	}
}
