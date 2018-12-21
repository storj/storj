// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"errors"
	"sync"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Mutex wraps DB in a mutex for non-concurrent databases
type Mutex struct {
	mu sync.Mutex
	db *DB
}

// NewMutex returns a mutex around DB
func NewMutex(db *DB) *Mutex {
	return &Mutex{db: db}
}

func (db *Mutex) locked() func() {
	db.mu.Lock()
	return db.mu.Unlock
}

// StatDB is a getter for StatDB repository
func (db *Mutex) StatDB() statdb.DB {
	return &muStatDB{mu: db, db: db.db.StatDB()}
}

// OverlayCache is a getter for overlay cache repository
func (db *Mutex) OverlayCache() storage.KeyValueStore {
	return db.db.OverlayCache()
}

// RepairQueue is a getter for RepairQueue repository
func (db *Mutex) RepairQueue() queue.RepairQueue {
	return db.db.RepairQueue()
}

// Accounting returns database for tracking bandwidth agreements over time
func (db *Mutex) Accounting() accounting.DB {
	return db.db.Accounting()
}

// Irreparable returns database for storing segments that failed repair
func (db *Mutex) Irreparable() irreparable.DB {
	return db.db.Irreparable()
}

// CreateTables is a method for creating all tables for database
func (db *Mutex) CreateTables() error {
	return db.db.CreateTables()
}

// Close is used to close db connection
func (db *Mutex) Close() error {
	return db.db.Close()
}

// muStatDB implements mutex around statdb.DB
type muStatDB struct {
	mu *Mutex
	db statdb.DB
}

// Create a db entry for the provided storagenode
func (mu *muStatDB) Create(ctx context.Context, nodeID storj.NodeID, startingStats *statdb.NodeStats) (stats *statdb.NodeStats, err error) {
	defer mu.mu.locked()()
	stats, err = mu.db.Create(ctx, nodeID, startingStats)
	return
}

// Get a storagenode's stats from the db
func (mu *muStatDB) Get(ctx context.Context, nodeID storj.NodeID) (stats *statdb.NodeStats, err error) {
	defer mu.mu.locked()()
	return mu.db.Get(ctx, nodeID)
}

// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements
func (mu *muStatDB) FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *statdb.NodeStats) (invalidIDs storj.NodeIDList, err error) {
	defer mu.mu.locked()()
	invalidIDs, err = mu.db.FindInvalidNodes(ctx, nodeIDs, maxStats)
	return
}

// Update all parts of single storagenode's stats in the db
func (mu *muStatDB) Update(ctx context.Context, updateReq *statdb.UpdateRequest) (stats *statdb.NodeStats, err error) {
	defer mu.mu.locked()()
	return mu.db.Update(ctx, updateReq)
}

// UpdateUptime updates a single storagenode's uptime stats in the db
func (mu *muStatDB) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *statdb.NodeStats, err error) {
	defer mu.mu.locked()()
	return mu.db.UpdateUptime(ctx, nodeID, isUp)
}

// UpdateAuditSuccess updates a single storagenode's audit stats in the db
func (mu *muStatDB) UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (stats *statdb.NodeStats, err error) {
	defer mu.mu.locked()()
	return mu.db.UpdateAuditSuccess(ctx, nodeID, auditSuccess)
}

// UpdateBatch for updating multiple farmers' stats in the db
func (mu *muStatDB) UpdateBatch(ctx context.Context, updateReqList []*statdb.UpdateRequest) (statsList []*statdb.NodeStats, failedUpdateReqs []*statdb.UpdateRequest, err error) {
	defer mu.mu.locked()()
	return mu.db.UpdateBatch(ctx, updateReqList)
}

// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
func (mu *muStatDB) CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (stats *statdb.NodeStats, err error) {
	defer mu.mu.locked()()
	stats, err = mu.db.CreateEntryIfNotExists(ctx, nodeID)
	return
}

type muOverlayCache struct {
	mu *Mutex
	db storage.KeyValueStore
}

func (db *muOverlayCache) Put(key storage.Key, value storage.Value) error {
	defer db.mu.locked()()
	return db.db.Put(key, value)
}

func (db *muOverlayCache) Get(key storage.Key) (storage.Value, error) {
	defer db.mu.locked()()
	return db.db.Get(key)
}

func (db *muOverlayCache) GetAll(keys storage.Keys) (storage.Values, error) {
	defer db.mu.locked()()
	return db.db.GetAll(keys)
}

func (db *muOverlayCache) Delete(key storage.Key) error {
	defer db.mu.locked()()
	return db.db.Delete(key)
}

func (db *muOverlayCache) List(start storage.Key, limit int) (keys storage.Keys, err error) {
	defer db.mu.locked()()
	return db.db.List(start, limit)
}

// ReverseList lists all keys in revers order
func (db *muOverlayCache) ReverseList(start storage.Key, limit int) (storage.Keys, error) {
	defer db.mu.locked()()
	return db.db.ReverseList(start, limit)
}

// Iterate iterates over items based on opts
func (db *muOverlayCache) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	return errors.New("not implemented")
}

// Close closes the store
func (db *muOverlayCache) Close() error {
	defer db.mu.locked()()
	return db.db.Close()
}
