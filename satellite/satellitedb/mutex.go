// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"errors"
	"sync"
	"time"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
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

// BandwidthAgreement is a getter for bandwidth agreement repository
func (db *Mutex) BandwidthAgreement() bwagreement.DB {
	return &muBandwidthAgreement{mu: db, db: db.db.BandwidthAgreement()}
}

// // PointerDB is a getter for PointerDB repository
// func (db *Mutex) PointerDB() pointerdb.DB {
// 	return &pointerDB{db: db.db}
// }

// StatDB is a getter for StatDB repository
func (db *Mutex) StatDB() statdb.DB {
	return &muStatDB{mu: db, db: db.db.StatDB()}
}

// OverlayCache is a getter for overlay cache repository
func (db *Mutex) OverlayCache() storage.KeyValueStore {
	return &muOverlayCache{mu: db, db: db.db.OverlayCache()}
}

// RepairQueue is a getter for RepairQueue repository
func (db *Mutex) RepairQueue() queue.RepairQueue {
	return &muRepairQueue{mu: db, db: db.db.RepairQueue()}
}

// Accounting returns database for tracking bandwidth agreements over time
func (db *Mutex) Accounting() accounting.DB {
	return &muAccounting{mu: db, db: db.db.Accounting()}
}

// Irreparable returns database for storing segments that failed repair
func (db *Mutex) Irreparable() irreparable.DB {
	return &muIrreparable{mu: db, db: db.db.Irreparable()}
}

// CreateTables is a method for creating all tables for database
func (db *Mutex) CreateTables() error {
	return db.db.CreateTables()
}

// Close is used to close db connection
func (db *Mutex) Close() error {
	return db.db.Close()
}

type muBandwidthAgreement struct {
	mu *Mutex
	db bwagreement.DB
}

func (db *muBandwidthAgreement) CreateAgreement(ctx context.Context, agreement bwagreement.Agreement) error {
	defer db.mu.locked()()
	return db.db.CreateAgreement(ctx, agreement)
}

func (db *muBandwidthAgreement) GetAgreements(ctx context.Context) ([]bwagreement.Agreement, error) {
	defer db.mu.locked()()
	return db.db.GetAgreements(ctx)
}

func (db *muBandwidthAgreement) GetAgreementsSince(ctx context.Context, since time.Time) ([]bwagreement.Agreement, error) {
	defer db.mu.locked()()
	return db.db.GetAgreementsSince(ctx, since)
}

// muStatDB implements mutex around statdb.DB
type muStatDB struct {
	mu *Mutex
	db statdb.DB
}

// Create a db entry for the provided storagenode
func (db *muStatDB) Create(ctx context.Context, nodeID storj.NodeID, startingStats *statdb.NodeStats) (stats *statdb.NodeStats, err error) {
	defer db.mu.locked()()
	return db.db.Create(ctx, nodeID, startingStats)
}

// Get a storagenode's stats from the db
func (db *muStatDB) Get(ctx context.Context, nodeID storj.NodeID) (stats *statdb.NodeStats, err error) {
	defer db.mu.locked()()
	return db.db.Get(ctx, nodeID)
}

// FindInvalidNodes finds a subset of storagenodes that have stats below provided reputation requirements
func (db *muStatDB) FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *statdb.NodeStats) (invalidIDs storj.NodeIDList, err error) {
	defer db.mu.locked()()
	return db.db.FindInvalidNodes(ctx, nodeIDs, maxStats)
}

// Update all parts of single storagenode's stats in the db
func (db *muStatDB) Update(ctx context.Context, updateReq *statdb.UpdateRequest) (stats *statdb.NodeStats, err error) {
	defer db.mu.locked()()
	return db.db.Update(ctx, updateReq)
}

// UpdateUptime updates a single storagenode's uptime stats in the db
func (db *muStatDB) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *statdb.NodeStats, err error) {
	defer db.mu.locked()()
	return db.db.UpdateUptime(ctx, nodeID, isUp)
}

// UpdateAuditSuccess updates a single storagenode's audit stats in the db
func (db *muStatDB) UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (stats *statdb.NodeStats, err error) {
	defer db.mu.locked()()
	return db.db.UpdateAuditSuccess(ctx, nodeID, auditSuccess)
}

// UpdateBatch for updating multiple farmers' stats in the db
func (db *muStatDB) UpdateBatch(ctx context.Context, updateReqList []*statdb.UpdateRequest) (statsList []*statdb.NodeStats, failedUpdateReqs []*statdb.UpdateRequest, err error) {
	defer db.mu.locked()()
	return db.db.UpdateBatch(ctx, updateReqList)
}

// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
func (db *muStatDB) CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (stats *statdb.NodeStats, err error) {
	defer db.mu.locked()()
	return db.db.CreateEntryIfNotExists(ctx, nodeID)
}

// muOverlayCache implements a mutex around overlay cache
type muOverlayCache struct {
	mu *Mutex
	db storage.KeyValueStore
}

// Put adds a value to store
func (db *muOverlayCache) Put(key storage.Key, value storage.Value) error {
	defer db.mu.locked()()
	return db.db.Put(key, value)
}

// Get gets a value to store
func (db *muOverlayCache) Get(key storage.Key) (storage.Value, error) {
	defer db.mu.locked()()
	return db.db.Get(key)
}

// GetAll gets all values from the store
func (db *muOverlayCache) GetAll(keys storage.Keys) (storage.Values, error) {
	defer db.mu.locked()()
	return db.db.GetAll(keys)
}

// Delete deletes key and the value
func (db *muOverlayCache) Delete(key storage.Key) error {
	defer db.mu.locked()()
	return db.db.Delete(key)
}

// List lists all keys starting from start and upto limit items
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

// muRepairQueue implements mutex around repair queue
type muRepairQueue struct {
	mu *Mutex
	db queue.RepairQueue
}

func (db *muRepairQueue) Enqueue(ctx context.Context, seg *pb.InjuredSegment) error {
	defer db.mu.locked()()
	return db.db.Enqueue(ctx, seg)
}
func (db *muRepairQueue) Dequeue(ctx context.Context) (pb.InjuredSegment, error) {
	defer db.mu.locked()()
	return db.db.Dequeue(ctx)
}
func (db *muRepairQueue) Peekqueue(ctx context.Context, limit int) ([]pb.InjuredSegment, error) {
	defer db.mu.locked()()
	return db.db.Peekqueue(ctx, limit)
}

type muAccounting struct {
	mu *Mutex
	db accounting.DB
}

func (db *muAccounting) LastRawTime(ctx context.Context, timestampType string) (time.Time, bool, error) {
	defer db.mu.locked()()
	return db.db.LastRawTime(ctx, timestampType)
}

func (db *muAccounting) SaveBWRaw(ctx context.Context, latestBwa time.Time, bwTotals map[string]int64) (err error) {
	defer db.mu.locked()()
	return db.db.SaveBWRaw(ctx, latestBwa, bwTotals)
}

func (db *muAccounting) SaveAtRestRaw(ctx context.Context, latestTally time.Time, nodeData map[storj.NodeID]int64) error {
	defer db.mu.locked()()
	return db.db.SaveAtRestRaw(ctx, latestTally, nodeData)
}

type muIrreparable struct {
	mu *Mutex
	db irreparable.DB
}

func (db *muIrreparable) IncrementRepairAttempts(ctx context.Context, segmentInfo *irreparable.RemoteSegmentInfo) (err error) {
	defer db.mu.locked()()
	return db.db.IncrementRepairAttempts(ctx, segmentInfo)
}

func (db *muIrreparable) Get(ctx context.Context, segmentPath []byte) (resp *irreparable.RemoteSegmentInfo, err error) {
	defer db.mu.locked()()
	return db.db.Get(ctx, segmentPath)
}

func (db *muIrreparable) Delete(ctx context.Context, segmentPath []byte) (err error) {
	defer db.mu.locked()()
	return db.db.Delete(ctx, segmentPath)
}
