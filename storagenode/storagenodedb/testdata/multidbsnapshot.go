// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import (
	"context"
	"fmt"

	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/sqliteutil"
)

// States is the global variable that stores all the states for testing.
var States = MultiDBStates{
	List: []*MultiDBState{
		&v0,
		&v1,
		&v2,
		&v3,
		&v4,
		&v5,
		&v6,
		&v7,
		&v8,
		&v9,
		&v10,
		&v11,
		&v12,
		&v13,
		&v14,
		&v15,
		&v16,
		&v17,
		&v18,
		&v19,
		&v20,
		&v21,
		&v22,
		&v23,
		&v24,
		&v25,
		&v26,
		&v27,
		&v28,
		&v29,
		&v30,
		&v31,
		&v32,
		&v33,
		&v34,
		&v35,
		&v36,
		&v37,
		&v38,
		&v39,
		&v40,
		&v41,
		&v42,
		&v43,
		&v44,
		&v45,
		&v46,
		&v47,
		&v48,
		&v49,
		&v50,
		&v51,
		&v52,
		&v53,
		&v54,
		&v55,
		&v56,
		&v57,
		&v58,
		&v59,
		&v60,
		&v61,
		&v62,
	},
}

// MultiDBStates provides a convenient list of MultiDBState.
type MultiDBStates struct {
	List []*MultiDBState
}

// FindVersion finds a MultiDBState with the specified version.
func (mdbs *MultiDBStates) FindVersion(version int) (*MultiDBState, bool) {
	for _, state := range mdbs.List {
		if state.Version == version {
			return state, true
		}
	}
	return nil, false
}

// MultiDBState represents an expected state across multiple DBs, defined in SQL
// commands.
type MultiDBState struct {
	Version  int
	DBStates DBStates
}

// DBStates is a convenience type.
type DBStates map[string]*DBState

// DBState allows you to define the desired state of the DB using SQl commands.
// Both the SQl and NewData fields contains SQL that will be executed to create
// the expected DB. The NewData SQL additionally will be executed on the testDB
// to ensure data is consistent. If OldData is not empty, it is executed on the
// testDB before the migration is run, and NewData is not run on the testDB. This
// is used to assert that a migration that modifies data runs as expected.
type DBState struct {
	SQL     string
	OldData string
	NewData string
}

// MultiDBSnapshot represents an expected state among multiple databases.
type MultiDBSnapshot struct {
	Version     int
	DBSnapshots DBSnapshots
}

// NewMultiDBSnapshot returns a new MultiDBSnapshot.
func NewMultiDBSnapshot() *MultiDBSnapshot {
	return &MultiDBSnapshot{
		DBSnapshots: DBSnapshots{},
	}
}

// DBSnapshots is a convenience type.
type DBSnapshots map[string]*DBSnapshot

// DBSnapshot is a snapshot of a single DB.
type DBSnapshot struct {
	Schema *dbschema.Schema
	Data   *dbschema.Data
}

// LoadMultiDBSnapshot converts a MultiDBState into a MultiDBSnapshot. It
// executes the SQL and stores the schema and data.
func LoadMultiDBSnapshot(ctx context.Context, multiDBState *MultiDBState) (*MultiDBSnapshot, error) {
	multiDBSnapshot := NewMultiDBSnapshot()
	for dbName, dbState := range multiDBState.DBStates {
		snapshot, err := sqliteutil.LoadSnapshotFromSQL(ctx, fmt.Sprintf("%s\n%s", dbState.SQL, dbState.NewData))
		if err != nil {
			return nil, err
		}
		multiDBSnapshot.DBSnapshots[dbName] = &DBSnapshot{
			Schema: snapshot.Schema,
			Data:   snapshot.Data,
		}
	}
	return multiDBSnapshot, nil
}
