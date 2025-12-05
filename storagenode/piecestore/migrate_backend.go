// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"encoding/json"
	"io/fs"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/satstore"
)

// MigrationState keeps track of the migration state for a satellite.
//
// N.B. we rely on the zero value meaning "no migration".
type MigrationState struct {
	PassiveMigrate bool // passive migrate pieces on read
	WriteToNew     bool // should writes go to the new store
	ReadNewFirst   bool // should reads go to the new or old store first
	TTLToNew       bool // any TTL write should go to the new store
}

// Migrator is an interface for migrating pieces.
type Migrator interface {
	TryMigrateOne(sat storj.NodeID, piece storj.PieceID)
}

// MigratingBackend is a PieceBackend that can passively migrate pieces
// from an OldPieceBackend to a HashStoreBackend.
type MigratingBackend struct {
	old      *OldPieceBackend
	new      *HashStoreBackend
	store    *satstore.SatelliteStore
	migrator Migrator
	states   atomic.Pointer[map[storj.NodeID]MigrationState]

	updateMu sync.Mutex // ensure one UpdateState call at a time
}

// NewMigratingBackend constructs a MigratingBackend with the given parameters.
func NewMigratingBackend(log *zap.Logger, old *OldPieceBackend, new *HashStoreBackend, store *satstore.SatelliteStore, migrator Migrator, contactService *contact.Service, suppressCentralMigration bool) *MigratingBackend {
	if log == nil {
		log = zap.NewNop()
	}
	mb := &MigratingBackend{
		old:      old,
		new:      new,
		store:    store,
		migrator: migrator,
	}

	states := make(map[storj.NodeID]MigrationState)
	err := store.Range(func(satellite storj.NodeID, data []byte) error {
		var ms MigrationState
		err := json.Unmarshal(data, &ms)
		if err != nil {
			log.Warn("failed to unmarshal migration state", zap.Error(err), zap.Stringer("satellite", satellite))
		}
		states[satellite] = ms
		return err
	})
	if err != nil {
		log.Warn("failed to iterate over migration state directories", zap.Error(err))
	}

	mb.states.Store(&states)

	if !suppressCentralMigration {
		contactService.RegisterCheckinCallback(func(ctx context.Context, satelliteID storj.NodeID, resp *pb.CheckInResponse) error {
			if resp.HashstoreSettings == nil {
				return nil
			}
			settings := resp.HashstoreSettings
			mb.UpdateState(ctx, satelliteID, func(state *MigrationState) {
				state.PassiveMigrate = state.PassiveMigrate || settings.PassiveMigrate
				state.WriteToNew = state.WriteToNew || settings.WriteToNew
				state.ReadNewFirst = state.ReadNewFirst || settings.ReadNewFirst
				state.TTLToNew = state.TTLToNew || settings.TtlToNew
			})
			return nil
		})
	}

	return mb
}

// Stats implements monkit.StatSource.
func (m *MigratingBackend) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	type IDState struct {
		id    storj.NodeID
		state MigrationState
	}

	states := *m.states.Load()
	idStates := make([]IDState, 0, len(states))
	for id, state := range states {
		idStates = append(idStates, IDState{id, state})
	}

	sort.Slice(idStates, func(i, j int) bool {
		return idStates[i].id.Less(idStates[j].id)
	})

	for _, idst := range idStates {
		monkit.StatSourceFromStruct(
			monkit.NewSeriesKey("migrating_backend").WithTag("satellite", idst.id.String()),
			idst.state,
		).Stats(cb)
	}
}

// UpdateState calls the callback with the current MigrationState for the satellite allowing the caller to inspect or modify the state.
func (m *MigratingBackend) UpdateState(ctx context.Context, satellite storj.NodeID, cb func(state *MigrationState)) {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()

	// load the states map and state for the satellite, keeping track of if it exists.
	states := *m.states.Load()
	state, exists := states[satellite]

	// keep track of the current state and call the callback, then compute if the state has changed.
	beforeCallback := state
	cb(&state)
	changed := beforeCallback != state

	// make a copy of the current state map into a new one and update the value for the satellite.
	next := make(map[storj.NodeID]MigrationState, len(states))
	for k, v := range states {
		next[k] = v
	}
	next[satellite] = state

	// publish the new immutable state map.
	m.states.Store(&next)

	// if data changed or we have a new entry, persist it to disk for next process start.
	if changed || !exists {
		data, _ := json.Marshal(state)        // impossible to error
		_ = m.store.Set(ctx, satellite, data) // ignore errors
	}
}

func (m *MigratingBackend) getState(ctx context.Context, satellite storj.NodeID) MigrationState {
	if state, ok := (*m.states.Load())[satellite]; ok {
		return state
	}
	m.UpdateState(ctx, satellite, func(*MigrationState) {}) // cause the state on disk to be updated.
	return MigrationState{}
}

// Writer implements PieceBackend by writing to the store appropriate for the migration status.
func (m *MigratingBackend) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, hash pb.PieceHashAlgorithm, expires time.Time) (_ PieceWriter, err error) {
	defer mon.Task()(&ctx)(&err)

	if state := m.getState(ctx, satellite); state.WriteToNew || (state.TTLToNew && !expires.IsZero()) {
		return m.new.Writer(ctx, satellite, pieceID, hash, expires)
	}
	return m.old.Writer(ctx, satellite, pieceID, hash, expires)
}

// Reader implements PieceBackend by reading from the store appropriate for the migration status, potentially
// triggering a passive migration for the piece.
func (m *MigratingBackend) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (r PieceReader, err error) {
	defer mon.Task()(&ctx)(&err)

	state := m.getState(ctx, satellite)

	// so, we potentially read from new twice to avoid a situation where a piece is being migrated
	// where if we only checked new and then old, we could 1. check new and miss, 2. migrate the
	// piece from old to new 3. check old and miss and oops we lost the piece. by checking new
	// before and after, this can't happen.
	if state.ReadNewFirst {
		r, err := m.new.Reader(ctx, satellite, pieceID)
		if err == nil || !errs.Is(err, fs.ErrNotExist) {
			return r, err
		}
	}
	if r, err := m.old.Reader(ctx, satellite, pieceID); err == nil {
		// try to migrate the piece if we're in passive migrate mode and we have a migrator.
		if state.PassiveMigrate && m.migrator != nil {
			m.migrator.TryMigrateOne(satellite, pieceID)
		}
		return r, nil
	}
	return m.new.Reader(ctx, satellite, pieceID)
}

// StartRestore implements PieceBackend and triggers a restore on both backends.
func (m *MigratingBackend) StartRestore(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task(monkit.NewSeriesTag("satellite", satellite.String()))(&ctx)(&err)

	return errs.Combine(
		m.new.StartRestore(ctx, satellite), // the hash store backend's start restore call does not block very long
		m.old.StartRestore(ctx, satellite),
	)
}
