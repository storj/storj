// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"github.com/zeebo/mwc"
	"go.uber.org/zap"

	"storj.io/drpc/drpcsignal"
)

const (
	db_MaxLoad     = 0.9 // maximum load factor of store before blocking new writes
	db_CompactLoad = 0.7 // load factor before starting compaction
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
	if runtime.GOOS == "windows" {
		return nil, errs.New("hashstore is not available on windows")
	}

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
		return nil, errs.Wrap(err)
	}
	d.passive, err = NewStore(filepath.Join(dir, "s1"), log.With(zap.String("store", "s1")))
	if err != nil {
		return nil, errs.Wrap(err)
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
	Load     float64 // number of set entries divided by number of slots in the hash table.
	NumSlots uint64  // total number of slots in the hash table.
	NumSet   uint64  // number of set entries in the hash table.
	AvgSize  float64 // average size of pieces in the database.

	NumLogs     uint64  // total number of log files.
	LogAlive    uint64  // number of bytes in the log files that are alive (records point to them).
	LogTotal    uint64  // total number of bytes in the log files.
	LogFraction float64 // percent of bytes that are alive in the log files.
	TableSize   uint64  // total number of bytes in the hash table.

	Compacting bool // if true, a background compaction is in progress.
	Active     int  // which store is currently active

	S0 StoreStats // statistics about the s0 store.
	S1 StoreStats // statistics about the s1 store.
}

// Stats returns statistics about the database.
func (d *DB) Stats() DBStats {
	d.mu.Lock()
	s0, s1, active := d.active, d.passive, 0
	compacting := d.compact != nil
	d.mu.Unlock()

	// sort them so s0 and s1 always get the same tag values.
	if s1.dir < s0.dir {
		s0, s1, active = s1, s0, 1
	}

	s0stats := s0.Stats()
	s1stats := s1.Stats()

	nslots := s0stats.NumSlots + s1stats.NumSlots
	nset := s0stats.NumSet + s1stats.NumSet

	numLogs := s0stats.NumLogs + s1stats.NumLogs
	logAlive := s0stats.LogAlive + s1stats.LogAlive
	logTotal := s0stats.LogTotal + s1stats.LogTotal
	logFraction := float64(logAlive) / float64(logTotal)
	tableSize := s0stats.TableSize + s1stats.TableSize

	return DBStats{
		Load:     float64(nset) / float64(nslots),
		NumSlots: nslots,
		NumSet:   nset,
		AvgSize:  float64(logAlive) / float64(nset),

		NumLogs:     numLogs,
		LogAlive:    logAlive,
		LogTotal:    logTotal,
		LogFraction: logFraction,
		TableSize:   tableSize,

		Compacting: compacting,
		Active:     active,

		S0: s0stats,
		S1: s1stats,
	}
}

// Close closes down the database and blocks until all background processes have stopped.
func (d *DB) Close() {
	d.cloMu.Lock()
	defer d.cloMu.Unlock()

	if !d.closed.Set(errs.New("db closed")) {
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
func (d *DB) Create(ctx context.Context, key Key, expires time.Time) (*Writer, error) {
	active, err := d.getActive(ctx)
	if err != nil {
		return nil, err
	}
	return active.Create(ctx, key, expires)
}

// Read returns a reader for the given key. If the key is not present the returned Reader will be
// nil. Close must be called on the reader when done.
func (d *DB) Read(ctx context.Context, key Key) (*Reader, error) {
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
	return second.Read(ctx, key)
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
		// if the store isn't already compacting, and it has been at least two days since it was
		// created, then we should compact it. note: it is 2 days it would be 1 day right after
		// midnight. this ensures it's at least 1 day old.
		return !stats.Compacting && stats.Today-stats.Created >= 2
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
	err := compact.store.Compact(ctx, d.shouldTrash, d.lastRestore(ctx))
	if err != nil {
		d.log.Error("compaction failed", zap.Error(err))
	}

	compact.cancel()
	compact.done.Set(err)

	d.mu.Lock()
	d.compact = nil
	d.mu.Unlock()
}
