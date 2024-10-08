// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/drpc/drpcsignal"
)

const (
	db_MaxLoad     = 0.9 // maximum load factor of store before blocking new writes
	db_CompactLoad = 0.5 // load factor before starting compaction
)

type compactState struct {
	store  *store
	cancel func()
	done   drpcsignal.Signal // set when compaction is done
}

// DB is a database that stores pieces.
type DB struct {
	dir         string
	nlogs       int
	log         *zap.Logger
	shouldTrash func(context.Context, Key, time.Time) (bool, error)
	lastRestore func(context.Context) (time.Time, error)

	closed drpcsignal.Signal // closed state
	cloMu  sync.Mutex        // synchronizes closing

	mu      sync.Mutex    // protects the following fields
	compact *compactState // set if compaction is in progress
	active  *store        // store that currently absorbs writes
	passive *store        // store that was being compacted
}

// New makes or opens an existing database in the directory allowing for nlogs concurrent writes.
func New(
	dir string, nlogs int, log *zap.Logger,
	shouldTrash func(context.Context, Key, time.Time) (bool, error),
	lastRestore func(context.Context) (time.Time, error),
) (*DB, error) {
	dir0 := filepath.Join(dir, "d0")
	dir1 := filepath.Join(dir, "d1")

	if err := os.MkdirAll(dir0, 0755); err != nil {
		return nil, errs.Wrap(err)
	}
	if err := os.MkdirAll(dir1, 0755); err != nil {
		return nil, errs.Wrap(err)
	}

	s0, err := newStore(dir0, nlogs, log)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	s1, err := newStore(dir1, nlogs, log)
	if err != nil {
		s0.Close()
		return nil, errs.Wrap(err)
	}

	// make the store with the larger load active. this is so that we have more time in the other
	// store before it needs compacting when the active store eventually starts compacting. it uses
	// <= instead of < only because it slightly increases code coverage (we do the swap for empty
	// databases) at ~zero cost.
	if s0.Load() <= s1.Load() {
		s0, s1 = s1, s0
	}

	db := &DB{
		dir:         dir,
		nlogs:       nlogs,
		log:         log,
		shouldTrash: shouldTrash,
		lastRestore: lastRestore,

		active:  s0,
		passive: s1,
	}

	// if the passive store's load is too high, immediately begin compacting it. this will allow us
	// to absorb writes more quickly if the active store becomes loaded.
	if db.passive.Load() >= db_CompactLoad {
		db.beginCompactingPassive()
	}

	return db, nil
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

	// if we have an active compaction, cancel and wait for it
	if compact != nil {
		compact.cancel()
		compact.done.Wait()
	}

	d.active.Close()
	d.passive.Close()
}

func (d *DB) getActive(ctx context.Context) (*store, error) {
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
		d.beginCompactingPassive()
	}

	return d.active, nil
}

func (d *DB) waitOnState(ctx context.Context, state *compactState) error {
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

func (d *DB) getReadPriority() (first, second *store, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.closed.Err(); err != nil {
		return nil, nil, err
	}

	// read from whichever store has more set keys first to give us a better chance of getting it
	// right. they should be about even, but this costs basically nothing to do, so might as well.
	first, second = d.active, d.passive
	if first.NumSet() < second.NumSet() {
		first, second = second, first
	}

	return first, second, nil
}

// Read returns a reader for the given key. If the key is not present the returned Reader will be
// nil. Close must be called on the reader when done.
func (d *DB) Read(ctx context.Context, key Key) (*Reader, error) {
	first, second, err := d.getReadPriority()
	if err != nil {
		return nil, err
	}

	r, err := first.Read(ctx, key)
	if err != nil {
		return nil, err
	} else if r != nil {
		return r, nil
	}
	return second.Read(ctx, key)
}

func (d *DB) beginCompactingPassive() {
	ctx, cancel := context.WithCancel(context.Background())
	d.compact = &compactState{
		store:  d.passive,
		cancel: cancel,
	}
	go d.performCompaction(ctx, d.compact)
}

func (d *DB) performCompaction(ctx context.Context, compact *compactState) {
	defer func() {
		compact.cancel()
		compact.done.Set(nil)

		d.mu.Lock()
		d.compact = nil
		d.mu.Unlock()
	}()

	err := func() (err error) {
		var lastRestore time.Time
		if d.lastRestore != nil {
			lastRestore, err = d.lastRestore(ctx)
			if err != nil {
				return errs.Wrap(err)
			}
		}

		if err := compact.store.Compact(ctx, d.shouldTrash, lastRestore); err != nil {
			return errs.Wrap(err)
		}

		return nil
	}()

	if err != nil {
		if d.log != nil {
			d.log.Error("compaction failed", zap.Error(err))
		}
		compact.done.Set(err)
	}
}
