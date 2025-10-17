// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"container/heap"
	"context"
	"encoding/binary"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

var _ DB = (*CachingDB)(nil)

// NewCachingDB creates a new CachingDB instance.
func NewCachingDB(log *zap.Logger, backingStore DirectDB, reputationConfig Config) *CachingDB {
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &CachingDB{
		log:                log,
		instanceOffset:     randSource.Uint64(),
		backingStore:       backingStore,
		nowFunc:            time.Now,
		reputationConfig:   reputationConfig,
		syncInterval:       reputationConfig.FlushInterval,
		errorRetryInterval: reputationConfig.ErrorRetryInterval,
		nextSyncTimer:      time.NewTimer(reputationConfig.FlushInterval),
		requestSyncChannel: make(chan syncRequest),
		pending:            make(map[storj.NodeID]*cachedNodeReputationInfo),
	}
}

// CachingDB acts like a reputation.DB but caches reads and writes, to minimize
// load on the backing store.
type CachingDB struct {
	// These fields must be populated before the cache starts being used.
	// They are not expected to change.
	log                *zap.Logger
	instanceOffset     uint64
	backingStore       DB
	nowFunc            func() time.Time
	reputationConfig   Config
	syncInterval       time.Duration
	errorRetryInterval time.Duration

	requestSyncChannel chan syncRequest

	// lock must be held when reading or writing to any of the following
	// fields.
	lock sync.Mutex

	nextSyncTimer *time.Timer

	// pending and writeOrderHeap contain the same set of entries, just with
	// different lookup properties. It should be easy to keep them in sync,
	// since we only insert with lock held, and (for now) we never evict
	// from the cache.
	pending        map[storj.NodeID]*cachedNodeReputationInfo
	writeOrderHeap nodeIDHeap
}

type syncRequest struct {
	nodeID   storj.NodeID
	doneChan chan struct{}
}

type cachedNodeReputationInfo struct {
	nodeID storj.NodeID

	// entryLock must be held when reading or writing to the following fields
	// in this structure (**except** syncAt. For syncAt, the CachingDB.lock
	// must be held). When entryLock is released, either info or syncError
	// (or both) must be non-nil.
	entryLock sync.Mutex

	// info is a best-effort copy of information from the database at some
	// point in the recent past (usually less than syncInterval ago) combined
	// with the requested updates which have not yet been synced to the
	// database.
	//
	// note: info has no guaranteed relationship to the set of mutations.
	// In particular, it is not necessarily the same as the base to which
	// the mutations will be applied.
	info *Info

	// syncError is the error that was encountered when trying to sync
	// info with the backing store. If this is set, errorRetryAt should also
	// be set.
	syncError error

	// errorRetryAt is the time at which a sync should be reattempted. It
	// should be set if syncError is set.
	errorRetryAt time.Time

	// syncAt is the time at which the system should try to apply the
	// pending mutations for this entry to the backing store. It should
	// be less than or equal to syncInterval from now.
	//
	// The corresponding CachingDB.lock must be held when reading from or
	// writing to this field.
	syncAt time.Time

	// mutations contains the set of changes to be made to a reputations
	// entry when the next sync operation fires.
	mutations Mutations
}

// Update applies a single update (one audit outcome) to a node's reputations
// record.
//
// If the node (as represented in the returned info) becomes newly vetted,
// disqualified, or suspended as a result of this update, the caller is
// responsible for updating the records in the overlay to match.
func (cdb *CachingDB) Update(ctx context.Context, request UpdateRequest, auditTime time.Time) (info *Info, err error) {
	defer mon.Task()(&ctx)(&err)

	mutations, err := UpdateRequestToMutations(request, auditTime)
	if err != nil {
		return nil, err
	}
	return cdb.ApplyUpdates(ctx, request.NodeID, mutations, request.Config, auditTime)
}

// ApplyUpdates applies multiple updates (defined by the updates parameter) to
// a node's reputations record.
//
// If the node (as represented in the returned info) becomes newly vetted,
// disqualified, or suspended as a result of these updates, the caller is
// responsible for updating the records in the overlay to match.
func (cdb *CachingDB) ApplyUpdates(ctx context.Context, nodeID storj.NodeID, updates Mutations, config Config, now time.Time) (info *Info, err error) {
	defer mon.Task()(&ctx)(&err)

	logger := cdb.log.With(zap.Stringer("node-id", nodeID))
	doRequestSync := false

	cdb.getEntry(ctx, nodeID, now, func(nodeEntry *cachedNodeReputationInfo) {
		if nodeEntry.syncError != nil {
			if ErrNodeNotFound.Has(nodeEntry.syncError) || errors.Is(nodeEntry.syncError, notPopulated) {
				// get it added to the database
				info, err = cdb.backingStore.ApplyUpdates(ctx, nodeID, updates, config, now)
				if err != nil {
					nodeEntry.syncError = err
					nodeEntry.errorRetryAt = now.Add(cdb.errorRetryInterval)
					return
				}
				nodeEntry.info = info.Copy()
				nodeEntry.syncError = nil
				return
			}
			err = nodeEntry.syncError
			return
		}

		if updates.OnlineHistory != nil {
			MergeAuditHistories(nodeEntry.mutations.OnlineHistory, updates.OnlineHistory.Windows, config.AuditHistory)
		}
		nodeEntry.mutations.PositiveResults += updates.PositiveResults
		nodeEntry.mutations.FailureResults += updates.FailureResults
		nodeEntry.mutations.OfflineResults += updates.OfflineResults
		nodeEntry.mutations.UnknownResults += updates.UnknownResults

		// We will also mutate the cached reputation info, as a best-effort
		// estimate of what the reputation should be when synced with the
		// backing store.
		cachedInfo := nodeEntry.info

		// We want to return a copy of this entity, after it has been mutated,
		// and the copy has to be done while we still hold the lock.
		defer func() { info = cachedInfo.Copy() }()

		trackingPeriodFull := false
		if updates.OnlineHistory != nil {
			trackingPeriodFull = MergeAuditHistories(cachedInfo.AuditHistory, updates.OnlineHistory.Windows, config.AuditHistory)
		}
		cachedInfo.AuditSuccessCount += int64(updates.PositiveResults)
		cachedInfo.TotalAuditCount += int64(updates.PositiveResults + updates.FailureResults + updates.OfflineResults + updates.UnknownResults)
		cachedInfo.OnlineScore = cachedInfo.AuditHistory.Score

		if cachedInfo.CreatedAt != nil {
			timeSinceCreation := now.Sub(*cachedInfo.CreatedAt)
			if cachedInfo.VettedAt == nil && timeSinceCreation >= config.MinimumNodeAge && cachedInfo.TotalAuditCount >= config.AuditCount {
				cachedInfo.VettedAt = &now
				// if we think the node is newly vetted, perform a sync to
				// have the best chance of propagating that information to
				// other satellite services.
				doRequestSync = true
			}
		}

		// for audit failure, only update normal alpha/beta
		cachedInfo.AuditReputationBeta, cachedInfo.AuditReputationAlpha = UpdateReputationMultiple(
			updates.FailureResults,
			cachedInfo.AuditReputationBeta,
			cachedInfo.AuditReputationAlpha,
			config.AuditLambda,
			config.AuditWeight,
		)
		// for audit unknown, only update unknown alpha/beta
		cachedInfo.UnknownAuditReputationBeta, cachedInfo.UnknownAuditReputationAlpha = UpdateReputationMultiple(
			updates.UnknownResults,
			cachedInfo.UnknownAuditReputationBeta,
			cachedInfo.UnknownAuditReputationAlpha,
			config.UnknownAuditLambda,
			config.AuditWeight,
		)

		// for a successful audit, increase reputation for normal *and* unknown audits
		cachedInfo.AuditReputationAlpha, cachedInfo.AuditReputationBeta = UpdateReputationMultiple(
			updates.PositiveResults,
			cachedInfo.AuditReputationAlpha,
			cachedInfo.AuditReputationBeta,
			config.AuditLambda,
			config.AuditWeight,
		)
		cachedInfo.UnknownAuditReputationAlpha, cachedInfo.UnknownAuditReputationBeta = UpdateReputationMultiple(
			updates.PositiveResults,
			cachedInfo.UnknownAuditReputationAlpha,
			cachedInfo.UnknownAuditReputationBeta,
			config.UnknownAuditLambda,
			config.AuditWeight,
		)

		mon.FloatVal("cached_audit_reputation_alpha").Observe(cachedInfo.AuditReputationAlpha)
		mon.FloatVal("cached_audit_reputation_beta").Observe(cachedInfo.AuditReputationBeta)
		mon.FloatVal("cached_unknown_audit_reputation_alpha").Observe(cachedInfo.UnknownAuditReputationAlpha)
		mon.FloatVal("cached_unknown_audit_reputation_beta").Observe(cachedInfo.UnknownAuditReputationBeta)
		mon.FloatVal("cached_audit_online_score").Observe(cachedInfo.OnlineScore)

		// The following code is all meant to keep the cache working
		// similarly to the values in the database. However, the cache
		// is not the "source of truth" and fields like Disqualified,
		// UnknownAuditSuspended, and UnderReview might be different
		// from what is in the backing store. If that happens, the cache
		// will get synced back to the source of truth the next time
		// this node is synchronized.

		// update audit score
		newAuditScore := cachedInfo.AuditReputationAlpha / (cachedInfo.AuditReputationAlpha + cachedInfo.AuditReputationBeta)
		// disqualification case a
		//   a) Success/fail audit reputation falls below audit DQ threshold
		if newAuditScore <= config.AuditDQ {
			if cachedInfo.Disqualified == nil {
				cachedInfo.Disqualified = &now
				cachedInfo.DisqualificationReason = overlay.DisqualificationReasonAuditFailure
				logger.Info("Disqualified", zap.String("dq-type", "audit failure"))
				// if we think the node is newly disqualified, perform a sync
				// to have the best chance of propagating that information to
				// other satellite services.
				doRequestSync = true
			}
		}

		// check unknown-audits score
		unknownAuditRep := cachedInfo.UnknownAuditReputationAlpha / (cachedInfo.UnknownAuditReputationAlpha + cachedInfo.UnknownAuditReputationBeta)
		if unknownAuditRep <= config.UnknownAuditDQ {
			if cachedInfo.UnknownAuditSuspended == nil {
				logger.Info("Suspended", zap.String("category", "unknown-result audits"))
				cachedInfo.UnknownAuditSuspended = &now
			}

			// disqualification case b
			//   b) Node is suspended (success/unknown reputation below audit DQ threshold)
			//        AND the suspended grace period has elapsed
			//        AND audit outcome is unknown or failed

			// if suspended grace period has elapsed and unknown audit rep is still
			// too low, disqualify node. Set suspended to nil if node is disqualified
			if cachedInfo.UnknownAuditSuspended != nil &&
				now.Sub(*cachedInfo.UnknownAuditSuspended) > config.SuspensionGracePeriod &&
				config.SuspensionDQEnabled {
				logger.Info("Disqualified", zap.String("dq-type", "suspension grace period expired for unknown-result audits"))
				cachedInfo.Disqualified = &now
				cachedInfo.DisqualificationReason = overlay.DisqualificationReasonSuspension
				cachedInfo.UnknownAuditSuspended = nil
			}
		} else if cachedInfo.UnknownAuditSuspended != nil {
			logger.Info("Suspension lifted", zap.String("category", "unknown-result audits"))
			cachedInfo.UnknownAuditSuspended = nil
		}

		// if suspension not enabled, skip penalization and unsuspend node if applicable
		if !config.AuditHistory.OfflineSuspensionEnabled {
			if cachedInfo.OfflineSuspended != nil {
				cachedInfo.OfflineSuspended = nil
			}
			if cachedInfo.UnderReview != nil {
				cachedInfo.UnderReview = nil
			}
			return
		}

		// only penalize node if online score is below threshold and
		// if it has enough completed windows to fill a tracking period
		penalizeOfflineNode := false
		if cachedInfo.OnlineScore < config.AuditHistory.OfflineThreshold && trackingPeriodFull {
			penalizeOfflineNode = true
		}

		// Suspension and disqualification for offline nodes
		if cachedInfo.UnderReview != nil {
			// move node in and out of suspension as needed during review period
			if !penalizeOfflineNode && cachedInfo.OfflineSuspended != nil {
				cachedInfo.OfflineSuspended = nil
			} else if penalizeOfflineNode && cachedInfo.OfflineSuspended == nil {
				cachedInfo.OfflineSuspended = &now
			}

			gracePeriodEnd := cachedInfo.UnderReview.Add(config.AuditHistory.GracePeriod)
			trackingPeriodEnd := gracePeriodEnd.Add(config.AuditHistory.TrackingPeriod)
			trackingPeriodPassed := now.After(trackingPeriodEnd)

			// after tracking period has elapsed, if score is good, clear under review
			// otherwise, disqualify node (if OfflineDQEnabled feature flag is true)
			if trackingPeriodPassed {
				if penalizeOfflineNode {
					if config.AuditHistory.OfflineDQEnabled {
						logger.Info("Disqualified", zap.String("dq-type", "node offline"))
						cachedInfo.Disqualified = &now
						cachedInfo.DisqualificationReason = overlay.DisqualificationReasonNodeOffline
					}
				} else {
					logger.Info("Suspension lifted", zap.String("category", "node offline"))
					cachedInfo.UnderReview = nil
					cachedInfo.OfflineSuspended = nil
				}
			}
		} else if penalizeOfflineNode {
			// suspend node for being offline and begin review period
			cachedInfo.UnderReview = &now
			cachedInfo.OfflineSuspended = &now
		}
	})

	if doRequestSync {
		_ = cdb.RequestSync(ctx, nodeID)
	}
	return info, err
}

// UnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
func (cdb *CachingDB) UnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = cdb.backingStore.UnsuspendNodeUnknownAudit(ctx, nodeID)
	if err != nil {
		return err
	}
	// sync with database (this will get it marked as unsuspended in the cache)
	return cdb.RequestSync(ctx, nodeID)
}

// DisqualifyNode disqualifies a storage node.
func (cdb *CachingDB) DisqualifyNode(ctx context.Context, nodeID storj.NodeID, disqualifiedAt time.Time, reason overlay.DisqualificationReason) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = cdb.backingStore.DisqualifyNode(ctx, nodeID, disqualifiedAt, reason)
	if err != nil {
		return err
	}
	// sync with database (this will get it marked as disqualified in the cache)
	return cdb.RequestSync(ctx, nodeID)
}

// SuspendNodeUnknownAudit suspends a storage node for unknown audits.
func (cdb *CachingDB) SuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = cdb.backingStore.SuspendNodeUnknownAudit(ctx, nodeID, suspendedAt)
	if err != nil {
		return err
	}
	// sync with database (this will get it marked as suspended in the cache)
	return cdb.RequestSync(ctx, nodeID)
}

// RequestSync requests the managing goroutine to perform a sync of cached info
// about the specified node to the backing store. This involves applying the
// cached mutations and resetting the info attribute to match a snapshot of what
// is in the backing store after the mutations.
func (cdb *CachingDB) RequestSync(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	req := syncRequest{
		nodeID:   nodeID,
		doneChan: make(chan struct{}, 1),
	}
	select {
	case cdb.requestSyncChannel <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-req.doneChan:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// FlushAll syncs all pending reputation mutations to the backing store.
func (cdb *CachingDB) FlushAll(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var copyOfEntries []*cachedNodeReputationInfo
	func() {
		cdb.lock.Lock()
		defer cdb.lock.Unlock()

		copyOfEntries = make([]*cachedNodeReputationInfo, 0, len(cdb.pending))
		for _, entry := range cdb.pending {
			copyOfEntries = append(copyOfEntries, entry)
		}
	}()

	var errg errs.Group
	for _, entry := range copyOfEntries {
		errg.Add(func() error {
			entry.entryLock.Lock()
			defer entry.entryLock.Unlock()

			cdb.syncEntry(ctx, entry, cdb.nowFunc())
			return entry.syncError
		}())
	}
	return errg.Err()
}

// Run runs the cache.
// NOTE: Run is automatically called by mud framework, but Manage doesn't.
func (cdb *CachingDB) Run(ctx context.Context) error {
	return cdb.Manage(ctx)
}

// Manage should be run in its own goroutine while a CachingDB is in use. This
// will schedule database flushes, trying to avoid too much load all at once.
func (cdb *CachingDB) Manage(ctx context.Context) error {
	for {
		select {
		case <-cdb.nextSyncTimer.C:
			cdb.syncDueEntries(ctx, cdb.nowFunc())
			cdb.updateTimer(cdb.nowFunc(), false)
		case request := <-cdb.requestSyncChannel:
			cdb.syncNode(ctx, request, cdb.nowFunc())
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// must not be called while there is a concurrent receive on
// cdb.nextSyncTimer.C (see the docs for time.(*Timer).Reset()).
//
// Here we achieve this requirement by calling this function only
// from the same goroutine that waits on that timer.
func (cdb *CachingDB) updateTimer(now time.Time, drainChannel bool) {
	cdb.lock.Lock()
	defer cdb.lock.Unlock()

	var timeToNextSync time.Duration
	if cdb.writeOrderHeap.Len() == 0 {
		// We could use any large-ish duration here. We just need to
		// keep the timer channel valid and want to avoid spinning on
		// updateTimer() calls.
		timeToNextSync = cdb.syncInterval
	} else {
		nextSync := cdb.writeOrderHeap[0].syncAt
		timeToNextSync = nextSync.Sub(now) // note: may be negative
	}

	if drainChannel {
		if !cdb.nextSyncTimer.Stop() {
			<-cdb.nextSyncTimer.C
		}
	}
	cdb.nextSyncTimer.Reset(timeToNextSync)
}

// getExistingEntry looks up an entry in the pending mutations cache, locks it,
// and calls f with the entry while holding the lock. If there is no entry in
// the cache with the given nodeID, f is not called.
func (cdb *CachingDB) getExistingEntry(nodeID storj.NodeID, f func(entryToSync *cachedNodeReputationInfo)) {
	var entryToSync *cachedNodeReputationInfo
	func() {
		cdb.lock.Lock()
		defer cdb.lock.Unlock()

		entryToSync = cdb.pending[nodeID]
	}()
	if entryToSync == nil {
		mon.Event("writecache-asked-for-unknown-node")
		return
	}

	func() {
		entryToSync.entryLock.Lock()
		defer entryToSync.entryLock.Unlock()

		f(entryToSync)
	}()
}

func (cdb *CachingDB) syncNode(ctx context.Context, request syncRequest, now time.Time) {
	defer close(request.doneChan)
	cdb.getExistingEntry(request.nodeID, func(entryToSync *cachedNodeReputationInfo) {
		cdb.syncEntry(ctx, entryToSync, now)
	})
}

func (cdb *CachingDB) syncDueEntries(ctx context.Context, now time.Time) {
	cdb.getEntriesToSync(now, func(entryToSync *cachedNodeReputationInfo) {
		cdb.syncEntry(ctx, entryToSync, now)
	})
}

// getEntriesToSync constructs a list of all entries due for syncing, updates
// the syncAt time for each, then locks each one individually and calls f()
// once for each entry while holding its lock.
func (cdb *CachingDB) getEntriesToSync(now time.Time, f func(entryToSync *cachedNodeReputationInfo)) {
	var entriesToSync []*cachedNodeReputationInfo

	func() {
		cdb.lock.Lock()
		defer cdb.lock.Unlock()

		for {
			if cdb.writeOrderHeap.Len() == 0 {
				break
			}
			if !cdb.writeOrderHeap[0].syncAt.Before(now) {
				break
			}
			entryToSync := cdb.writeOrderHeap[0]

			// We bump syncAt regardless of whether we are about to sync. If
			// something else is already syncing this entry, it has taken
			// more time than expected, and the next flush is due. We need
			// cdb.writeOrderHeap[0].syncAt.After(now) before we can exit
			// from this loop.
			entryToSync.syncAt = cdb.nextTimeForSync(entryToSync.nodeID, now)

			// move element 0 to its new correct place in the heap. This
			// shouldn't affect entryToSync, which is a pointer to the
			// entry shared by the heap.
			heap.Fix(&cdb.writeOrderHeap, 0)

			entriesToSync = append(entriesToSync, entryToSync)
		}
	}()

	for len(entriesToSync) > 0 {
		entry := entriesToSync[0]
		func() {
			entry.entryLock.Lock()
			defer entry.entryLock.Unlock()

			f(entry)
		}()
		entriesToSync = entriesToSync[1:]
	}
}

func (cdb *CachingDB) nextTimeForSync(nodeID storj.NodeID, now time.Time) time.Time {
	return nextTimeForSync(cdb.instanceOffset, nodeID, now, cdb.syncInterval)
}

// nextTimeForSync decides the next time at which the given nodeID should next
// be synchronized with the backing store.
//
// We make an effort to distribute the nodes in time, so that the service
// is not usually trying to retrieve or update many rows at the same time. We
// also make an effort to offset this sync schedule by a random value unique
// to this process so that in most cases, instances will not be trying to
// update the same row at the same time, minimizing contention.
func nextTimeForSync(instanceOffset uint64, nodeID storj.NodeID, now time.Time, syncInterval time.Duration) time.Time {
	// calculate the fraction into the FlushInterval at which this node
	// should always be synchronized.
	initialPosition := binary.BigEndian.Uint64(nodeID[:8])
	finalPosition := initialPosition + instanceOffset
	positionAsFraction := float64(finalPosition) / (1 << 64)
	// and apply that fraction to the actual interval
	periodStart := now.Truncate(syncInterval)
	offsetFromStart := time.Duration(positionAsFraction * float64(syncInterval))
	syncTime := periodStart.Add(offsetFromStart)
	if syncTime.Before(now) {
		syncTime = syncTime.Add(syncInterval)
	}
	// reapply monotonic time by applying the time delta to 'now'
	timeToNextSync := syncTime.Sub(now)
	return now.Add(timeToNextSync)
}

// syncEntry synchronizes an entry with the backing store. Any pending mutations
// will be applied to the backing store, and the info and syncError attributes
// will be updated according to the results.
//
// syncEntry must be called with the entry already locked.
func (cdb *CachingDB) syncEntry(ctx context.Context, entry *cachedNodeReputationInfo, now time.Time) {
	defer mon.Task()(&ctx)(nil)

	entry.info, entry.syncError = cdb.backingStore.ApplyUpdates(ctx, entry.nodeID, entry.mutations, cdb.reputationConfig, now)

	// NOTE: If another process has been updating the same row in the
	// backing store, it is possible that the node has become newly vetted,
	// disqualified, or suspended without us knowing about it. In this case,
	// the overlay will not know about the change until it next updates the
	// reputation. We may need to add some way for this object to notify the
	// overlay of updates such as this.

	if entry.syncError != nil {
		if ErrNodeNotFound.Has(entry.syncError) {
			entry.errorRetryAt = now
		} else {
			entry.errorRetryAt = now.Add(cdb.errorRetryInterval)
		}
	}
	entry.mutations = Mutations{
		OnlineHistory: &pb.AuditHistory{},
	}
}

// Get retrieves the cached *Info record for the given node ID. If the
// information is not already in the cache, the information is fetched from the
// backing store.
//
// If an error occurred syncing the entry with the backing store, it will be
// returned. In this case, the returned value for 'info' might be nil, or it
// might contain data cached longer than FlushInterval.
func (cdb *CachingDB) Get(ctx context.Context, nodeID storj.NodeID) (info *Info, err error) {
	defer mon.Task()(&ctx)(&err)

	cdb.getEntry(ctx, nodeID, cdb.nowFunc(), func(entry *cachedNodeReputationInfo) {
		if entry.syncError != nil {
			err = entry.syncError
		}
		if entry.info != nil {
			info = entry.info.Copy()
		}
	})
	return info, err
}

// getEntry acquires an entry (a *cachedNodeReputationInfo) in the reputation
// cache, locks it, and supplies the entry to the given callback function for
// access or mutation. The pointer to the entry will not remain valid after the
// callback function returns.
//
// If there is no record for the requested nodeID, a new record will be added
// for it, it will be synced with the backing store, and the new record will be
// supplied to the given callback function.
//
// If there was an error fetching up-to-date info from the backing store, the
// entry supplied to the callback will have entry.syncError != nil. In this
// case, entry.info may be nil, or it may have an out-of-date record. If the
// error occurred long enough ago that it is time to try again, another attempt
// to sync the entry will occur before the callback is made.
func (cdb *CachingDB) getEntry(ctx context.Context, nodeID storj.NodeID, now time.Time, f func(entry *cachedNodeReputationInfo)) {
	defer mon.Task()(&ctx)(nil)

	var nodeEntry *cachedNodeReputationInfo

	func() {
		cdb.lock.Lock()
		defer cdb.lock.Unlock()

		var ok bool
		nodeEntry, ok = cdb.pending[nodeID]
		if !ok {
			nodeEntry = cdb.insertNode(nodeID, now)
		}
	}()

	func() {
		nodeEntry.entryLock.Lock()
		defer nodeEntry.entryLock.Unlock()

		if nodeEntry.syncError != nil && nodeEntry.errorRetryAt.Before(now) {
			cdb.syncEntry(ctx, nodeEntry, now)
		}

		f(nodeEntry)
	}()
}

// Inserts a mostly-empty *cachedNodeReputationInfo record into the pending
// list and the write-order heap.
//
// The syncError is pre-set so that the first caller to acquire the entryLock
// on the new entry should initiate an immediate sync with the backing store.
//
// cdb.lock must be held when calling.
func (cdb *CachingDB) insertNode(nodeID storj.NodeID, now time.Time) *cachedNodeReputationInfo {
	syncTime := cdb.nextTimeForSync(nodeID, now)
	mut := &cachedNodeReputationInfo{
		nodeID:       nodeID,
		syncAt:       syncTime,
		syncError:    notPopulated,
		errorRetryAt: time.Time{}, // sync will be initiated right away
		mutations: Mutations{
			OnlineHistory: &pb.AuditHistory{},
		},
	}

	cdb.pending[nodeID] = mut
	heap.Push(&cdb.writeOrderHeap, mut)

	return mut
}

// SetNowFunc supplies a new function to use for determining the current time,
// for synchronization timing and scheduling purposes. This is frequently useful
// in test scenarios.
func (cdb *CachingDB) SetNowFunc(timeFunc func() time.Time) {
	cdb.nowFunc = timeFunc
}

// notPopulated is an error indicating that a cachedNodeReputationInfo
// structure has not yet been populated. The syncError field is initialized
// to this error, and the first access of the entry should cause an immediate
// lookup to the backing store. Therefore, this error should not normally
// escape outside writecache code.
var notPopulated = Error.New("not populated")

// nodeIDHeap is a heap of cachedNodeReputationInfo entries, ordered by the
// associated syncAt times. It implements heap.Interface.
type nodeIDHeap []*cachedNodeReputationInfo

// Len returns the length of the slice.
func (n nodeIDHeap) Len() int {
	return len(n)
}

// Swap swaps the elements with indices i and j.
func (n nodeIDHeap) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// Less returns true if the syncAt time for the element with index i comes
// before the syncAt time for the element with index j.
func (n nodeIDHeap) Less(i, j int) bool {
	return n[i].syncAt.Before(n[j].syncAt)
}

// Push appends an element to the slice.
func (n *nodeIDHeap) Push(x interface{}) {
	*n = append(*n, x.(*cachedNodeReputationInfo))
}

// Pop removes and returns the last element in the slice.
func (n *nodeIDHeap) Pop() interface{} {
	oldLen := len(*n)
	item := (*n)[oldLen-1]
	*n = (*n)[:oldLen-1]
	return item
}
