// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqueue

import (
	"container/heap"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/jobq"
)

const (
	// queueSelectMask can be ANDed with an index from indexByID to give
	// either inRepairQueue or inRetryQueue.
	queueSelectMask = uint64(1 << 63)
	// indexMask can be ANDed with an index from indexByID to give the
	// index alone, without the queue selection bit.
	indexMask = uint64((1 << 63) - 1) // all bits except the first

	// inRepairQueue indicates that an index is in the repair queue.
	inRepairQueue = uint64(0)
	// inRetryQueue indicates that an index is in the retry queue.
	inRetryQueue = uint64(1 << 63)
)

// jobQueue provides common functionality to repairPriorityQueue and
// repairRetryQueue.
type jobQueue struct {
	// priorityHeap is a priority queue implemented as a heap. Code MUST NOT
	// return this slice or slices of it callers of this library, as it may be
	// over memory not managed by Go.
	priorityHeap []jobq.RepairJob
	// mem points to an arbitrary byte slice, or nil, as determined by the
	// platform-specific memory management code. It may point to the same memory
	// as priorityHeap, but as a byte slice of the full range. This would be
	// used to release memory when the heap shrinks or reallocate it when it
	// grows too large, to minimize the number of places we have to use `unsafe`
	// functions.
	mem []byte
	// indexByID is a map of streamID+position to the index in the priority heap
	// where that job is stored. The index is shared by both queues, so its
	// values are stored as a uint64 with the first bit indicating which queue
	// the job is in (0 for repair, 1 for retry).
	indexByID map[jobq.SegmentIdentifier]uint64
	// memReleaseThreshold is the number of items that can be removed from the
	// heap before calling markUnused to release memory. In brief, we call
	// markUnused if highWater - len(priorityHeap) >= memReleaseThreshold.
	memReleaseThreshold int
	// highWater is the highest number of items that have been in the heap at
	// any time since the last call to markUnused.
	highWater int
	// unmarkingError contains an error returned by markUnused, if any. If it is
	// not nil, no further calls to markUnused will be made from this queue.
	unmarkingError error
	// queueSelect is a constant that indicates which queue this jobQueue is
	// associated with. It corresponds with the most significant bit of a
	// uint64, and is either 0 for the repair queue or 1 for the retry queue
	// (inRepairQueue and inRetryQueue, respectively). When storing indexes in
	// the indexByID map, this value is ORed with the index to indicate which
	// queue the job is in.
	queueSelect uint64
}

func newJobQueue(indexByID map[jobq.SegmentIdentifier]uint64, initAlloc int, memReleaseThreshold int, queueSelect uint64) (jobQueue, error) {
	mem, heapAll, err := memAlloc(initAlloc)
	if err != nil {
		return jobQueue{}, err
	}
	return jobQueue{
		priorityHeap:        heapAll,
		mem:                 mem,
		indexByID:           indexByID,
		memReleaseThreshold: memReleaseThreshold,
		queueSelect:         queueSelect,
	}, nil
}

func (jq *jobQueue) Len() int { return len(jq.priorityHeap) }

func (jq jobQueue) Swap(i, j int) {
	jq.priorityHeap[i], jq.priorityHeap[j] = jq.priorityHeap[j], jq.priorityHeap[i]
	jq.indexByID[jq.priorityHeap[i].ID] = uint64(i) | jq.queueSelect
	jq.indexByID[jq.priorityHeap[j].ID] = uint64(j) | jq.queueSelect
}

func (jq *jobQueue) Pop() any {
	item := jq.priorityHeap[len(jq.priorityHeap)-1]
	// don't keep calling markUnused if it's already failed once
	if jq.unmarkingError == nil {
		jq.highWater, jq.unmarkingError = markUnused(jq.mem, jq.priorityHeap, len(jq.priorityHeap)-1, jq.highWater, jq.memReleaseThreshold)
	}
	jq.priorityHeap = jq.priorityHeap[0 : len(jq.priorityHeap)-1]
	delete(jq.indexByID, item.ID)
	return item
}

func (jq *jobQueue) Push(x any) {
	item := x.(jobq.RepairJob)
	if len(jq.priorityHeap) == cap(jq.priorityHeap) {
		// we need to realloc the mmap'd memory
		var err error
		jq.mem, jq.priorityHeap, err = memRealloc(jq.mem, jq.priorityHeap, cap(jq.priorityHeap)*2)
		if err != nil {
			// If realloc fails, we will just panic. This is probably fine; most
			// of the ways this could fail should have caused a failure the
			// first time we mmap'd memory.
			panic(fmt.Sprintf("failed to realloc memory: %v", err))
		}
	}
	jq.priorityHeap = append(jq.priorityHeap, item)
	jq.indexByID[item.ID] = uint64(len(jq.priorityHeap)-1) | jq.queueSelect
	if len(jq.priorityHeap)-1 > jq.highWater {
		jq.highWater = len(jq.priorityHeap) - 1
	}
}

func (jq *jobQueue) Truncate() {
	jq.priorityHeap = jq.priorityHeap[:0]
	if jq.unmarkingError == nil {
		jq.highWater, jq.unmarkingError = markUnused(jq.mem, jq.priorityHeap, 0, jq.highWater, jq.memReleaseThreshold)
	}
}

func (jq *jobQueue) cleanQueue(updatedBefore time.Time) (removed int) {
	updatedBeforeUnix := uint64(updatedBefore.Unix())
	removed = 0
	for i := 0; i < len(jq.priorityHeap); {
		if jq.priorityHeap[i].UpdatedAt < updatedBeforeUnix {
			// remove item by swapping it with the end and shortening the slice
			if i != len(jq.priorityHeap)-1 {
				jq.priorityHeap[i], jq.priorityHeap[len(jq.priorityHeap)-1] = jq.priorityHeap[len(jq.priorityHeap)-1], jq.priorityHeap[i]
			}
			jq.priorityHeap = jq.priorityHeap[:len(jq.priorityHeap)-1]
			removed++
		} else {
			i++
		}
	}
	if jq.unmarkingError == nil {
		jq.highWater, jq.unmarkingError = markUnused(jq.mem, jq.priorityHeap, len(jq.priorityHeap), jq.highWater, jq.memReleaseThreshold)
	}
	// must be followed up with a heap.Init() and a reindex of jq.indexByID to
	// maintain heap properties.
	return removed
}

func (jq *jobQueue) trimQueue(healthGreaterThan float64) (removed int) {
	removed = 0
	for i := 0; i < len(jq.priorityHeap); {
		if jq.priorityHeap[i].Health > healthGreaterThan {
			// remove item by swapping it with the end and shortening the slice
			if i != len(jq.priorityHeap)-1 {
				jq.priorityHeap[i], jq.priorityHeap[len(jq.priorityHeap)-1] = jq.priorityHeap[len(jq.priorityHeap)-1], jq.priorityHeap[i]
			}
			jq.priorityHeap = jq.priorityHeap[:len(jq.priorityHeap)-1]
			removed++
		} else {
			i++
		}
	}
	if jq.unmarkingError == nil {
		jq.highWater, jq.unmarkingError = markUnused(jq.mem, jq.priorityHeap, len(jq.priorityHeap), jq.highWater, jq.memReleaseThreshold)
	}
	// must be followed up with a heap.Init() and a reindex of jq.indexByID to
	// maintain heap properties.
	return removed
}

type repairPriorityQueue struct {
	jobQueue
}

func (rpq *repairPriorityQueue) Less(i, j int) bool {
	return rpq.priorityHeap[i].Health < rpq.priorityHeap[j].Health
}

var _ heap.Interface = &repairPriorityQueue{}

type repairRetryQueue struct {
	jobQueue
	// headChan is a channel that will be written to anytime the first element
	// in the retry queue changes, so that the funnel goroutine can wake up and
	// update its wait timer if necessary.
	headChan chan<- struct{}
	// stoppedChan is a channel that will be closed when the funnel goroutine
	// stops.
	stoppedChan <-chan struct{}
}

func (rrq *repairRetryQueue) Less(i, j int) bool {
	return rrq.priorityHeap[i].LastAttemptedAt < rrq.priorityHeap[j].LastAttemptedAt
}

func (rrq *repairRetryQueue) Swap(i, j int) {
	rrq.jobQueue.Swap(i, j)
	if (i == 0 || j == 0) && rrq.headChan != nil {
		rrq.headChan <- struct{}{}
	}
}

func (rrq *repairRetryQueue) Push(x any) {
	rrq.jobQueue.Push(x)
	if rrq.Len() == 1 && rrq.headChan != nil {
		rrq.headChan <- struct{}{}
	}
}

var _ heap.Interface = &repairRetryQueue{}

// Queue is a priority queue of repair jobs paired with a priority queue of jobs
// to be retried once they are eligible. A secondary index on streamID+position
// is kept to allow updates to the health (priority) of jobs already in one of
// the queues.
type Queue struct {
	lock sync.Mutex
	log  *zap.Logger
	pq   repairPriorityQueue
	rq   repairRetryQueue
	// indexByID is a map of streamID+position to the index in the priority heap
	// where that job is stored. The index is shared by both queues, so its values
	// are stored as a uint64 with the first bit indicating which queue the job is
	// in (0 for repair, 1 for retry).
	indexByID map[jobq.SegmentIdentifier]uint64

	RetryAfter time.Duration
	Now        func() time.Time
}

// NewQueue creates a new Queue.
func NewQueue(log *zap.Logger, retryAfter time.Duration, initialAlloc, memReleaseThreshold int) (*Queue, error) {
	indexByID := make(map[jobq.SegmentIdentifier]uint64)
	pqJobQueue, err := newJobQueue(indexByID, initialAlloc, memReleaseThreshold, inRepairQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap repair priority queue: %w", err)
	}
	rqJobQueue, err := newJobQueue(indexByID, initialAlloc, memReleaseThreshold, inRetryQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap repair retry queue: %w", err)
	}
	return &Queue{
		log:        log,
		pq:         repairPriorityQueue{jobQueue: pqJobQueue},
		rq:         repairRetryQueue{jobQueue: rqJobQueue},
		indexByID:  indexByID,
		RetryAfter: retryAfter,
		Now:        time.Now,
	}, nil
}

// Insert adds a job to the queue with the given health. If the segment
// is already in the repair queue or the retry queue, the job record is
// updated and left in the queue (with its position updated as necessary)
//
// When a job is updated, its InsertedAt value is preserved, its UpdatedAt
// field is set to the current time, and the new NumAttempts field is added to
// the previously existing value.
//
// If the job is not already in either queue and its LastAttemptedAt field is
// recent enough (as determined by RetryAfter), it is added to the retry queue
// instead of the repair queue, to wait until it is eligible for another try.
//
// Returns true if the job was newly added to a queue, and false if an existing
// entry in the target queue was updated.
func (q *Queue) Insert(job jobq.RepairJob) (wasNew bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	// if the segment is already in the queue, we can't tell with the heap alone
	// (without some O(N) searching). indexByID is here for this reason.
	if i, ok := q.indexByID[job.ID]; ok {
		index := int(i & indexMask)
		targetQueue := &q.pq.jobQueue
		var targetHeap heap.Interface = &q.pq
		if i&queueSelectMask == inRetryQueue {
			targetQueue = &q.rq.jobQueue
			targetHeap = &q.rq
		}
		fixNeeded := false
		oldJob := targetQueue.priorityHeap[index]
		if oldJob.Health != job.Health {
			fixNeeded = true
		}
		job.NumAttempts += oldJob.NumAttempts
		job.InsertedAt = oldJob.InsertedAt
		job.UpdatedAt = uint64(q.Now().Unix())
		targetQueue.priorityHeap[index] = job
		// only need to fix the position in the heap if the health changed
		if fixNeeded {
			heap.Fix(targetHeap, index)
		}
		return false
	}
	if job.InsertedAt == 0 {
		job.InsertedAt = uint64(q.Now().Unix())
	}
	job.UpdatedAt = uint64(q.Now().Unix())

	if job.LastAttemptedAt != 0 && q.Now().Sub(job.LastAttemptedAtTime()) < q.RetryAfter {
		// new job, but not eligible for retry yet
		heap.Push(&q.rq, job)
	} else {
		// new job, can be repaired immediately
		heap.Push(&q.pq, job)
	}
	return true
}

// Pop removes and returns the segment with the lowest health from the repair
// queue. If there are no segments in the queue, it returns a zero job and
// ok=false.
func (q *Queue) Pop() (job jobq.RepairJob, ok bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.popLocked()
}

func (q *Queue) popLocked() (job jobq.RepairJob, ok bool) {
	if q.pq.Len() == 0 {
		return jobq.RepairJob{}, false
	}

	unmarkingErrorBefore := q.pq.unmarkingError
	item := heap.Pop(&q.pq).(jobq.RepairJob)
	if unmarkingErrorBefore == nil && q.pq.unmarkingError != nil {
		q.log.Error("failed to mark unused memory", zap.Error(q.pq.unmarkingError))
	}
	return item, true
}

// Peek returns the segment with the lowest health without removing it from
// the queue. If there are no segments in the queue, it returns a zero UUID and
// position.
func (q *Queue) Peek() (job jobq.RepairJob, ok bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.pq.Len() == 0 {
		return jobq.RepairJob{}, false
	}

	return q.pq.priorityHeap[0], true
}

// PeekRetry returns the segment with the smallest LastUpdatedAt value in the
// retry queue without removing it from the queue. If there are no segments in
// the queue, it returns a zero UUID and position.
func (q *Queue) PeekRetry() jobq.RepairJob {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.rq.Len() == 0 {
		return jobq.RepairJob{}
	}

	return q.rq.priorityHeap[0]
}

// Len returns the number of segments in the repair queue and the retry queue,
// respectively.
func (q *Queue) Len() (inRepair, inRetry int64) {
	q.lock.Lock()
	defer q.lock.Unlock()

	return int64(q.pq.Len()), int64(q.rq.Len())
}

// Delete removes a segment from the queue by streamID and position, whether it
// is in the repair queue or the retry queue. Returns true if the segment was
// found and removed, and false if it was not found.
func (q *Queue) Delete(streamID uuid.UUID, position uint64) (wasDeleted bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if i, ok := q.indexByID[jobq.SegmentIdentifier{StreamID: streamID, Position: position}]; ok {
		index := int(i & indexMask)
		targetQueue := &q.pq.jobQueue
		var targetHeap heap.Interface = &q.pq
		if i&queueSelectMask == inRetryQueue {
			targetQueue = &q.rq.jobQueue
			targetHeap = &q.rq
		}
		if index < targetQueue.Len() {
			heap.Remove(targetHeap, index)
		}
		return true
	}
	return false
}

// Inspect finds a repair job in the queue by streamID and position and returns
// all of the job information.
func (q *Queue) Inspect(streamID uuid.UUID, position uint64) jobq.RepairJob {
	q.lock.Lock()
	defer q.lock.Unlock()

	if i, ok := q.indexByID[jobq.SegmentIdentifier{StreamID: streamID, Position: position}]; ok {
		if i&queueSelectMask == inRetryQueue {
			return q.rq.priorityHeap[int(i&indexMask)]
		}
		return q.pq.priorityHeap[int(i&indexMask)]
	}
	return jobq.RepairJob{}
}

const checkForCancelEvery = 1000

// Stat performs some analysis of the items in the queue and returns some
// related statistics. This is a relatively expensive operation at O(n). The
// queues for this placement are left locked for the duration of the operation;
// all reads and writes to this queue will block until this is complete.
func (q *Queue) Stat(ctx context.Context) (repairStat, retryStat jobq.QueueStat, err error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	repairStat.Count = int64(q.pq.Len())
	retryStat.Count = int64(q.rq.Len())
	var maxInsertedAt, minInsertedAt uint64
	var maxAttemptedAt, minAttemptedAt *uint64
	first := true
	updateStat := func(item jobq.RepairJob, stat *jobq.QueueStat) {
		if first || item.InsertedAt > maxInsertedAt {
			maxInsertedAt = item.InsertedAt
		}
		if first || item.InsertedAt < minInsertedAt {
			minInsertedAt = item.InsertedAt
		}
		if item.LastAttemptedAt != 0 && (maxAttemptedAt == nil || item.LastAttemptedAt > *maxAttemptedAt) {
			t := item.LastAttemptedAt
			maxAttemptedAt = &t
		}
		if item.LastAttemptedAt != 0 && (minAttemptedAt == nil || item.LastAttemptedAt < *minAttemptedAt) {
			t := item.LastAttemptedAt
			minAttemptedAt = &t
		}
		if first || item.Health > stat.MaxSegmentHealth {
			stat.MaxSegmentHealth = item.Health
		}
		if first || item.Health < stat.MinSegmentHealth {
			stat.MinSegmentHealth = item.Health
		}
	}

	for i, item := range q.pq.priorityHeap {
		updateStat(item, &repairStat)
		first = false
		if i%checkForCancelEvery == 0 {
			if err := ctx.Err(); err != nil {
				return repairStat, retryStat, err
			}
		}
	}
	repairStat.MaxInsertedAt = time.Unix(int64(maxInsertedAt), 0)
	repairStat.MinInsertedAt = time.Unix(int64(minInsertedAt), 0)
	if maxAttemptedAt != nil {
		t := time.Unix(int64(*maxAttemptedAt), 0)
		repairStat.MaxAttemptedAt = &t
	}
	if minAttemptedAt != nil {
		t := time.Unix(int64(*minAttemptedAt), 0)
		repairStat.MinAttemptedAt = &t
	}

	maxInsertedAt, minInsertedAt = 0, 0
	maxAttemptedAt, minAttemptedAt = nil, nil
	first = true
	for i, item := range q.rq.priorityHeap {
		updateStat(item, &retryStat)
		first = false
		if i%checkForCancelEvery == 0 {
			if err := ctx.Err(); err != nil {
				return repairStat, retryStat, err
			}
		}
	}
	retryStat.MaxInsertedAt = time.Unix(int64(maxInsertedAt), 0)
	retryStat.MinInsertedAt = time.Unix(int64(minInsertedAt), 0)
	if maxAttemptedAt != nil {
		t := time.Unix(int64(*maxAttemptedAt), 0)
		retryStat.MaxAttemptedAt = &t
	}
	if minAttemptedAt != nil {
		t := time.Unix(int64(*minAttemptedAt), 0)
		retryStat.MinAttemptedAt = &t
	}

	return repairStat, retryStat, nil
}

// Truncate removes all items currently in the queue.
func (q *Queue) Truncate() {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.pq.Truncate()
	q.rq.Truncate()
	maps.Clear(q.indexByID)
}

// Clean removes all items from the queues that were last updated before the
// given time. This is a relatively expensive operation at O(n). The queues for
// this placement are left locked for the duration of the operation; all reads
// and writes to this placement will block until this is complete.
//
// Returns the total number of items removed from the queues.
func (q *Queue) Clean(updatedBefore time.Time) (removed int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	maps.Clear(q.indexByID)
	removed += q.pq.cleanQueue(updatedBefore)
	removed += q.rq.cleanQueue(updatedBefore)
	heap.Init(&q.pq)
	heap.Init(&q.rq)
	for i, item := range q.pq.priorityHeap {
		q.indexByID[item.ID] = uint64(i) | q.pq.queueSelect
	}
	for i, item := range q.rq.priorityHeap {
		q.indexByID[item.ID] = uint64(i) | q.rq.queueSelect
	}
	return removed
}

// Trim removes all items from the queues with health greater than the given
// value. This is a relatively expensive operation at O(n). The queues for this
// placement are left locked for the duration of the operation; all reads and
// writes to this placement will block until this is complete.
//
// Returns the total number of items removed from the queues.
func (q *Queue) Trim(healthGreaterThan float64) (removed int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	maps.Clear(q.indexByID)
	removed += q.pq.trimQueue(healthGreaterThan)
	removed += q.rq.trimQueue(healthGreaterThan)
	heap.Init(&q.pq)
	heap.Init(&q.rq)
	for i, item := range q.pq.priorityHeap {
		q.indexByID[item.ID] = uint64(i) | q.pq.queueSelect
	}
	for i, item := range q.rq.priorityHeap {
		q.indexByID[item.ID] = uint64(i) | q.rq.queueSelect
	}
	return removed
}

// TestingSetAttemptedTime sets the LastAttemptedAt field for a segment in the
// queue by streamID and position. It returns the number of jobs affected (this
// will be 0 or 1).
func (q *Queue) TestingSetAttemptedTime(streamID uuid.UUID, position uint64, lastAttemptedAt time.Time) (rowsAffected int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if i, ok := q.indexByID[jobq.SegmentIdentifier{StreamID: streamID, Position: position}]; ok {
		index := int(i & indexMask)
		targetQueue := &q.pq.jobQueue
		if i&queueSelectMask == inRetryQueue {
			targetQueue = &q.rq.jobQueue
		}
		targetQueue.priorityHeap[index].LastAttemptedAt = uint64(lastAttemptedAt.Unix())
		return 1
	}
	return 0
}

// Start starts the queue's funnel goroutine, which moves items from the retry
// queue to the repair queue as they become eligible for retry (after RetryAfter).
// If the queue is already running, it returns an error.
func (q *Queue) Start() error {
	q.lock.Lock()
	if q.rq.headChan != nil {
		q.lock.Unlock()
		return fmt.Errorf("the queue is already running")
	}
	stoppedChan := make(chan struct{})
	headChan := make(chan struct{}, 10)
	q.rq.headChan = headChan
	log := q.log
	q.rq.stoppedChan = stoppedChan
	q.lock.Unlock()

	go q.funnelFromRetryToRepair(log, headChan, stoppedChan)
	return nil
}

// Stop stops the queue's funnel goroutine.
func (q *Queue) Stop() {
	q.Truncate()

	q.lock.Lock()
	if q.rq.headChan != nil {
		close(q.rq.headChan)
		q.rq.headChan = nil
	}
	q.lock.Unlock()

	if q.rq.stoppedChan != nil {
		<-q.rq.stoppedChan
		q.rq.stoppedChan = nil
	}
}

// Destroy stops the queue's funnel goroutine (if it is still running) and frees
// the associated memory.
func (q *Queue) Destroy() {
	q.Stop()
	_ = memFree(q.pq.mem)
	q.pq.mem = nil
	q.pq.priorityHeap = nil
	_ = memFree(q.rq.mem)
	q.rq.mem = nil
	q.rq.priorityHeap = nil
}

// ResetTimer causes the funnel goroutine to wake up and adjust its wait timer
// (might be used after artificially changing the clock, for example).
func (q *Queue) ResetTimer() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.rq.headChan != nil {
		q.rq.headChan <- struct{}{}
		return nil
	}
	return fmt.Errorf("the queue is not running")
}

// FunnelFromRetryToRepair moves items from the retry queue to the repair queue
// as they become eligible for retry (after RetryAfter).
func (q *Queue) funnelFromRetryToRepair(log *zap.Logger, headChan <-chan struct{}, stoppedChan chan<- struct{}) {
	defer close(stoppedChan)
	log.Info("starting funnel from retry to repair")
	defer log.Info("stopping funnel from retry to repair")

	for {
		var timeToNext time.Duration
		func() {
			q.lock.Lock()
			defer q.lock.Unlock()

			timeToNext = time.Minute
			if q.rq.Len() == 0 {
				return
			}
			nextItem := q.rq.priorityHeap[0]
			nextRunTime := nextItem.LastAttemptedAtTime().Add(q.RetryAfter)
			timeToNext = nextRunTime.Sub(q.Now())
			if timeToNext <= 0 {
				// disable headChan reporting while we're modifying the queue
				tmpChan := q.rq.headChan
				q.rq.headChan = nil
				defer func() { q.rq.headChan = tmpChan }()

				item := heap.Pop(&q.rq).(jobq.RepairJob)
				heap.Push(&q.pq, item)
			}
		}()

		timer := time.NewTimer(timeToNext)
		select {
		case <-timer.C:
		case _, ok := <-headChan:
			timer.Stop()
			if !ok {
				return
			}
		}
	}
}

type queueAndPlacement struct {
	*Queue
	placement storj.PlacementConstraint
}

func sortQueueMap(queueMap map[storj.PlacementConstraint]*Queue) []queueAndPlacement {
	queues := make([]queueAndPlacement, 0, len(queueMap))
	for placement, queue := range queueMap {
		queues = append(queues, queueAndPlacement{Queue: queue, placement: placement})
	}
	sort.Slice(queues, func(i, j int) bool {
		return queues[i].placement < queues[j].placement
	})
	return queues
}

// PopNMultipleQueues removes and returns the 'limit' segments with the lowest
// health from any of the given queues without removing them from the queues. If
// there are fewer than 'limit' segments in all of the queues, it returns all
// available. Checks only the repair queues, not the retry queues.
//
// This function is useful for combining multiple queues into a single view of
// the lowest health segments across all of them. Older repair code expects a
// single queue containing all placements and all jobs whether eligible for
// retry or not, so this function allows similar usage. Hopefully soon we can
// teach the repair workers to ask for jobs from each placement separately.
func PopNMultipleQueues(limit int, queueMap map[storj.PlacementConstraint]*Queue) (jobs []jobq.RepairJob) {
	// We must first lock _all_ of the target queues, as we need a consistent
	// view of their heap arrays. Deadlock danger: if two goroutines are trying
	// to do this and the queues are locked in a different order, they will
	// deadlock. To ensure a common ordering, we sort the queues by their
	// associated placement number.
	queues := sortQueueMap(queueMap)

	// lock all queues in order
	for _, q := range queues {
		q.lock.Lock()
	}
	defer func() {
		for i := len(queues) - 1; i >= 0; i-- {
			queues[i].lock.Unlock()
		}
	}()

	jobs = make([]jobq.RepairJob, 0, limit)
	for i := 0; i < limit; i++ {
		// find the lowest health item across all queues
		var lowestHealth float64
		lowestIndex := -1
		for j, q := range queues {
			if len(q.pq.priorityHeap) == 0 {
				continue
			}
			job := q.pq.priorityHeap[0]
			if lowestIndex == -1 || job.Health < lowestHealth {
				lowestIndex = j
				lowestHealth = job.Health
			}
		}
		if lowestIndex == -1 {
			// all queues are empty
			break
		}
		nextJob, _ := queues[lowestIndex].popLocked()
		jobs = append(jobs, nextJob)
	}
	return jobs
}

// PeekNMultipleQueues returns the 'limit' segments with the lowest health from
// any of the given queues without removing them from the queues. If there are
// fewer than 'limit' segments in all of the queues, it returns all available.
// Checks only the repair queues, not the retry queues.
//
// This function is useful for combining multiple queues into a single view of
// the lowest health segments across all of them. Older repair code expects a
// single queue containing all placements and all jobs whether eligible for
// retry or not, so this function allows similar usage. This is not very
// performant, but as far as I can tell we only need this in test situations.
func PeekNMultipleQueues(limit int, queueMap map[storj.PlacementConstraint]*Queue) (jobs []jobq.RepairJob) {
	// We must first lock _all_ of the target queues, as we need a consistent
	// view of their heap arrays. Deadlock danger: if two goroutines are trying
	// to do this and the queues are locked in a different order, they may
	// deadlock. To ensure a common ordering, we sort the queues by their
	// associated placement number.
	queues := sortQueueMap(queueMap)

	// lock all queues in order
	for _, q := range queues {
		q.lock.Lock()
	}
	defer func() {
		for i := len(queues) - 1; i >= 0; i-- {
			queues[i].lock.Unlock()
		}
	}()

	// now we build "overlay heaps" for each queue
	overlays := make([]*overlayHeap, len(queues))
	for i, q := range queues {
		overlays[i] = newOverlayHeap(q.pq.priorityHeap, q.pq.Less)
	}

	jobs = make([]jobq.RepairJob, 0, limit)
	for i := 0; i < limit; i++ {
		// find the lowest health item across all queues
		var lowestHealth float64
		lowestIndex := -1
		for j, overlay := range overlays {
			if overlay.Len() == 0 {
				continue
			}
			job := overlay.Peek()
			if lowestIndex == -1 || job.Health < lowestHealth {
				lowestIndex = j
				lowestHealth = job.Health
			}
		}
		if lowestIndex == -1 {
			// all queues are empty
			break
		}
		nextJob := heap.Pop(overlays[lowestIndex]).(jobq.RepairJob)
		jobs = append(jobs, nextJob)
	}
	return jobs
}
