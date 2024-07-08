// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"strings"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

var (
	// ErrGCProgress represents errors from the filewalker database.
	ErrGCProgress = errs.Class("gc_filewalker_progress_db")

	maxPrefixesScannedToFlush = 5
)

// GCFilewalkerProgressDBName represents the database name.
const GCFilewalkerProgressDBName = "garbage_collection_filewalker_progress"

type gcFilewalkerProgressDB struct {
	dbContainerImpl

	mu  sync.Mutex
	buf map[storj.NodeID]*progressBuffer
}

type progressBuffer struct {
	prefixesScanned int
	pieces.GCFilewalkerProgress
}

func (db *gcFilewalkerProgressDB) Store(ctx context.Context, progress pieces.GCFilewalkerProgress) (err error) {
	defer mon.Task()(&ctx)(&err)

	db.mu.Lock()
	if db.buf == nil {
		db.buf = make(map[storj.NodeID]*progressBuffer)
	}
	sat := db.buf[progress.SatelliteID]

	if sat == nil {
		sat = &progressBuffer{}
		db.buf[progress.SatelliteID] = sat
	}
	sat.prefixesScanned++
	sat.GCFilewalkerProgress = progress
	if sat.prefixesScanned < maxPrefixesScannedToFlush {
		db.mu.Unlock()
		return nil
	}
	// if any of the satellites has scanned more than maxPrefixesScannedToFlush prefixes, flush the buffer
	batch := db.buf
	db.buf = make(map[storj.NodeID]*progressBuffer)
	db.mu.Unlock()

	var args []any
	for id, buf := range batch {
		args = append(args, id, buf.BloomfilterCreatedBefore.UTC(), buf.Prefix)
	}

	values := strings.TrimRight(strings.Repeat("(?,?,?),", len(args)/3), ",")
	_, err = db.ExecContext(ctx, `
		INSERT OR REPLACE INTO progress(satellite_id, bloomfilter_created_before, last_checked_prefix)
		VALUES `+values, args...)

	return ErrGCProgress.Wrap(err)
}

func (db *gcFilewalkerProgressDB) Get(ctx context.Context, satelliteID storj.NodeID) (progress pieces.GCFilewalkerProgress, err error) {
	defer mon.Task()(&ctx)(&err)

	db.mu.Lock()
	if db.buf != nil {
		if buf, ok := db.buf[satelliteID]; ok {
			defer db.mu.Unlock()
			return buf.GCFilewalkerProgress, nil
		}
	}
	db.mu.Unlock()

	err = db.QueryRowContext(ctx, `
		SELECT last_checked_prefix, bloomfilter_created_before
		FROM progress
		WHERE satellite_id = ?
	`, satelliteID).Scan(&progress.Prefix, &progress.BloomfilterCreatedBefore)

	progress.SatelliteID = satelliteID

	return progress, ErrGCProgress.Wrap(err)
}

func (db *gcFilewalkerProgressDB) Reset(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	db.mu.Lock()
	delete(db.buf, satelliteID)
	db.mu.Unlock()

	_, err = db.ExecContext(ctx, `
		DELETE FROM progress
		WHERE satellite_id = ?
	`, satelliteID)

	return ErrGCProgress.Wrap(err)
}
