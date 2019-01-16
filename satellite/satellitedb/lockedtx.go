// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"sync"

	"storj.io/storj/satellite/console"
)

// BeginTransaction is a method for opening transaction
func (m *lockedConsole) BeginTx(ctx context.Context) (console.DBTx, error) {
	m.Lock()
	db, err := m.db.BeginTx(ctx)

	txlocked := &lockedConsole{&sync.Mutex{}, db}
	return &lockedTx{m, txlocked, db, sync.Once{}}, err
}

// lockedTx extends Database with transaction scope
type lockedTx struct {
	parent *lockedConsole
	*lockedConsole
	tx   console.DBTx
	once sync.Once
}

// Commit is a method for committing and closing transaction
func (db *lockedTx) Commit() error {
	err := db.tx.Commit()
	db.once.Do(db.parent.Unlock)
	return err
}

// Rollback is a method for rollback and closing transaction
func (db *lockedTx) Rollback() error {
	err := db.tx.Rollback()
	db.once.Do(db.parent.Unlock)
	return err
}
