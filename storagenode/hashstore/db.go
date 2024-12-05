// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"github.com/zeebo/mwc"
	"go.uber.org/zap"

	"storj.io/drpc/drpcsignal"
)

// Error is the class that wraps all errors generated by the hashstore package.
var Error = errs.Class("hashstore")

const (
	db_MaxLoad     = 0.95 // maximum load factor of store before blocking new writes
	db_CompactLoad = 0.75 // load factor before starting compaction
)

type compactState struct {
	store  *Store
	cancel func()
	done   drpcsignal.Signal // set when compaction is done
}

// DB is a database that stores pieces.
type DB struct {
	dir         string
	log         *zap.Logger
	shouldTrash func(context.Context, Key, time.Time) bool
	lastRestore func(context.Context) time.Time

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing
	wg     sync.WaitGroup    // waitgroup for background goroutines

	mu      sync.Mutex    // protects the following fields
	compact *compactState // set if compaction is in progress
	active  *Store        // store that currently absorbs writes
	passive *Store        // store that was being compacted
}

// New makes or opens an existing database in the directory allowing for nlogs concurrent writes.
func New(
	dir string, log *zap.Logger,
	shouldTrash func(context.Context, Key, time.Time) bool,
	lastRestore func(context.Context) time.Time,
) (_ *DB, err error) {
	// set default values for the optional parameters.
	if log == nil {
		log = zap.NewNop()
	}
	if lastRestore == nil {
		lastRestore = func(ctx context.Context) time.Time { return time.Time{} }
	}

	// partially initialize the database so that we can close it if there's an error.
	d := &DB{
		dir:         dir,
		log:         log,
		shouldTrash: shouldTrash,
		lastRestore: lastRestore,
	}
	defer func() {
		if err != nil {
			d.Close()
		}
	}()

	// open the active and passive stores.
	d.active, err = NewStore(filepath.Join(dir, "s0"), log.With(zap.String("store", "s0")))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	d.passive, err = NewStore(filepath.Join(dir, "s1"), log.With(zap.String("store", "s1")))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// make the store with the larger load active. this is so that we have more time in the other
	// store before it needs compacting when the active store eventually starts compacting. it uses
	// <= instead of < only because it slightly increases code coverage (we do the swap for empty
	// databases) at ~zero cost.
	if d.active.Load() <= d.passive.Load() {
		d.active, d.passive = d.passive, d.active
	}

	// if the passive store's load is too high, immediately begin compacting it. this will allow us
	// to absorb writes more quickly if the active store becomes loaded.
	if d.passive.Load() >= db_CompactLoad {
		d.beginPassiveCompaction()
	}

	// start a background goroutine to ensure that the database compacts the store at least once
	// a day to have a mechanism to clean up ttl data even if no writes are occurring.
	d.wg.Add(1)
	go d.backgroundCompactions()

	return d, nil
}

// DBStats is a collection of statistics about a database.
type DBStats struct {
	NumSet uint64  // number of set records.
	LenSet uint64  // sum of lengths in set records.
	AvgSet float64 // average size of length of records.

	NumTrash uint64  // number of set trash records.
	LenTrash uint64  // sum of lengths in set trash records.
	AvgTrash float64 // average size of length of trash records.

	NumSlots  uint64  // total number of records available.
	TableSize uint64  // total number of bytes in the hash table.
	Load      float64 // percent of slots that are set.

	NumLogs    uint64 // total number of log files.
	LenLogs    uint64 // total number of bytes in the log files.
	NumLogsTTL uint64 // total number of log files with ttl set.
	LenLogsTTL uint64 // total number of bytes in log files with ttl set.

	SetPercent   float64 // percent of bytes that are set in the log files.
	TrashPercent float64 // percent of bytes that are trash in the log files.

	Compacting  bool   // if true, a background compaction is in progress.
	Compactions uint64 // total number of compactions that finished on either store.
	Active      int    // which store is currently active
}

// Stats returns statistics about the database and underlying stores.
func (d *DB) Stats() (DBStats, StoreStats, StoreStats) {
	d.mu.Lock()
	s0, s1, active := d.active, d.passive, 0
	compacting := d.compact != nil
	d.mu.Unlock()

	// sort them so s0 and s1 always get the same tag values.
	if s1.dir < s0.dir {
		s0, s1, active = s1, s0, 1
	}

	s0st := s0.Stats()
	s1st := s1.Stats()

	return DBStats{
		NumSet: s0st.Table.NumSet + s1st.Table.NumSet,
		LenSet: s0st.Table.LenSet + s1st.Table.LenSet,
		AvgSet: safeDivide(float64(s0st.Table.LenSet+s1st.Table.LenSet), float64(s0st.Table.NumSet+s1st.Table.NumSet)),

		NumTrash: s0st.Table.NumTrash + s1st.Table.NumTrash,
		LenTrash: s0st.Table.LenTrash + s1st.Table.LenTrash,
		AvgTrash: safeDivide(float64(s0st.Table.LenTrash+s1st.Table.LenTrash), float64(s0st.Table.NumTrash+s1st.Table.NumTrash)),

		NumSlots:  s0st.Table.NumSlots + s1st.Table.NumSlots,
		TableSize: s0st.Table.TableSize + s1st.Table.TableSize,
		Load:      safeDivide(float64(s0st.Table.NumSet+s1st.Table.NumSet), float64(s0st.Table.NumSlots+s1st.Table.NumSlots)),

		NumLogs:    s0st.NumLogs + s1st.NumLogs,
		LenLogs:    s0st.LenLogs + s1st.LenLogs,
		NumLogsTTL: s0st.NumLogsTTL + s1st.NumLogsTTL,
		LenLogsTTL: s0st.LenLogsTTL + s1st.LenLogsTTL,

		SetPercent:   safeDivide(float64(s0st.Table.LenSet+s1st.Table.LenSet), float64(s0st.LenLogs+s1st.LenLogs)),
		TrashPercent: safeDivide(float64(s0st.Table.LenTrash+s1st.Table.LenTrash), float64(s0st.LenLogs+s1st.LenLogs)),

		Compacting:  compacting,
		Compactions: s0st.Compactions + s1st.Compactions,
		Active:      active,
	}, s0st, s1st
}

// Close closes down the database and blocks until all background processes have stopped.
func (d *DB) Close() {
	d.cloMu.Lock()
	defer d.cloMu.Unlock()

	if !d.closed.Set(Error.New("db closed")) {
		return
	}

	d.mu.Lock()
	compact := d.compact
	d.mu.Unlock()

	// if we have an active compaction, cancel and wait for it.
	if compact != nil {
		compact.cancel()
		compact.done.Wait()
	}

	// close down the stores now that compaction is finished.
	if d.active != nil {
		d.active.Close()
	}
	if d.passive != nil {
		d.passive.Close()
	}

	// wait for any background goroutines to finish.
	d.wg.Wait()
}

func (d *DB) getActive(ctx context.Context) (*Store, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.closed.Err(); err != nil {
		return nil, err
	}

	for {
		// if our load is lower than the compact load, we're good to create.
		load := d.active.Load()
		if load < db_CompactLoad {
			break
		}

		// check if we have a compaction going.
		if state := d.compact; state != nil {
			// if the load is low enough, we can still insert into the active store.
			if load < db_MaxLoad {
				break
			}

			// otherwise, the insertion may fail, so wait for it to finish. we drop the lock so that
			// Close can proceed, but this means that by the time we relock, we have to recheck the
			// load factor which is why this is a for loop.
			d.mu.Unlock()
			err := d.waitOnState(ctx, state)
			d.mu.Lock()

			if err != nil {
				return nil, err
			}
			continue
		}

		// no compaction in progress already when one is indicated by the load, so swap active and
		// begin the compaction.
		d.active, d.passive = d.passive, d.active
		d.beginPassiveCompaction()
	}

	return d.active, nil
}

func (d *DB) waitOnState(ctx context.Context, state *compactState) (err error) {
	defer mon.Task()(&ctx)(&err)

	// check if we're already closed so we don't have to worry about select nondeterminism: a closed
	// db or already canceled context will definitely error.
	if err := d.closed.Err(); err != nil {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.closed.Signal():
		return d.closed.Err()
	case <-state.done.Signal():
		return nil
	}
}

// Create adds an entry to the database with the given key and expiration time. Close or Cancel
// must be called on the Writer when done. It is safe to call either of them multiple times.
func (d *DB) Create(ctx context.Context, key Key, expires time.Time) (_ *Writer, err error) {
	defer mon.Task()(&ctx)(&err)

	active, err := d.getActive(ctx)
	if err != nil {
		return nil, err
	}
	return active.Create(ctx, key, expires)
}

// Read returns a reader for the given key. If the key is not present the returned Reader will be
// nil and the error will be a wrapped fs.ErrNotExist. Close must be called on the non-nil Reader
// when done.
func (d *DB) Read(ctx context.Context, key Key) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := d.closed.Err(); err != nil {
		return nil, err
	}

	d.mu.Lock()
	first, second := d.active, d.passive
	d.mu.Unlock()

	r, err := first.Read(ctx, key)
	if err != nil {
		return nil, err
	} else if r != nil {
		return r, nil
	}

	r, err = second.Read(ctx, key)
	if err != nil {
		return nil, err
	} else if r != nil {
		return r, nil
	}

	return nil, Error.Wrap(fs.ErrNotExist)
}

// Compact waits for any background compaction to finish and then calls Compact on both stores.
// After a call to Compact, you can be sure that each Store was fully compacted at least once.
func (d *DB) Compact(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := d.closed.Err(); err != nil {
		return err
	}

again:
	d.mu.Lock()
	if compact := d.compact; compact != nil {
		// an active compaction is happening. we have to drop the mutex and wait for some event to
		// let us proceed before trying again.
		d.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.closed.Signal():
			return d.closed.Err()
		case <-compact.done.Signal():
		}

		goto again
	}
	// we have the lock with no active compaction, so we can compact the Stores. we drop the lock
	// so that Close can still interrupt us. concurrent reads should be able to proceed with no
	// problem, but concurrent writes may conflict with the compact call and take longer.
	active, passive := d.active, d.passive
	d.mu.Unlock()

	lastRestore := d.lastRestore(ctx)
	return errs.Combine(
		active.Compact(ctx, d.shouldTrash, lastRestore),
		passive.Compact(ctx, d.shouldTrash, lastRestore),
	)
}

func (d *DB) beginPassiveCompaction() {
	// sanity check: don't overwrite an existing compaction. this is a programmer error. we don't
	// panic or anything because the code kinda assumes that the stores are arbitrarily loaded in
	// many places, so skipping this compaction isn't the end of the world.
	if d.compact != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.compact = &compactState{
		store:  d.passive,
		cancel: cancel,
	}
	go d.performPassiveCompaction(ctx, d.compact)
}

// backgroundCompactions polls periodically while the db is not closed to compact stores if they
// need it.
func (d *DB) backgroundCompactions() {
	defer d.wg.Done()

	for {
		select {
		case <-d.closed.Signal():
			return

		case <-time.After(time.Minute):
			// jitter background compactions so that they happen randomly through the day. since
			// we're checking once a minute, only actually do the check on average once a day.
			if mwc.Rand().Intn(24*60) == 0 {
				d.checkBackgroundCompactions()
			}
		}
	}
}

func (d *DB) checkBackgroundCompactions() {
	shouldCompact := func(s *Store) bool {
		stats := s.Stats()
		// if the store is already compacting, no need to start another compaction.
		if stats.Compacting {
			return false
		}
		// we require that the hash table be created long enough ago and that the last time we ran a
		// compaction be long enough ago. we check both because a compaction doesn't necessarily
		// rewrite the hash table if it detects there would be no modifications. we compare to a
		// value of 2 days because we want to ensure that it's been at least a full day since the
		// last compaction, and our granularity is only to the day (if it was 1, then if the last
		// compaction was right before midnight, we would immediately be able to compact again right
		// after midnight).
		return stats.Today-stats.LastCompact >= 2 && stats.Today-stats.Table.Created >= 2
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// if there's already a compaction going, don't start another one.
	if d.compact != nil {
		return
	}

	// compact the passive store if it needs it.
	if shouldCompact(d.passive) {
		d.beginPassiveCompaction()
		return
	}

	// compact the active store if it needs it.
	if shouldCompact(d.active) {
		d.active, d.passive = d.passive, d.active
		d.beginPassiveCompaction()
		return
	}
}

func (d *DB) performPassiveCompaction(ctx context.Context, compact *compactState) {
	var err error
	defer mon.Task()(&ctx)(&err)

	err = compact.store.Compact(ctx, d.shouldTrash, d.lastRestore(ctx))
	if err != nil {
		d.log.Error("compaction failed", zap.Error(err))
	}

	compact.cancel()
	compact.done.Set(err)

	d.mu.Lock()
	d.compact = nil
	d.mu.Unlock()
}
