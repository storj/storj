// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest

import (
	"fmt"
	"sort"

	"storj.io/storj/internal/dbutil/dbschema"
	"storj.io/storj/internal/dbutil/sqliteutil"
)

// Snapshots is the global variable that stores all the snapshots for testing
var Snapshots MultiDBSnapshots

// MultiDBSnapshots provides a convenient list of MultiDBSnapshot
type MultiDBSnapshots struct {
	List []*MultiDBSnapshot
}

// Add adds a new snapshot.
func (mdbs *MultiDBSnapshots) Add(snap *MultiDBSnapshot) *MultiDBSnapshot {
	mdbs.List = append(mdbs.List, snap)
	return snap
}

// FindVersion finds a snapshot with the specified version.
func (mdbs *MultiDBSnapshots) FindVersion(version int) (*MultiDBSnapshot, bool) {
	for _, snap := range mdbs.List {
		if snap.Version == version {
			return snap, true
		}
	}
	return nil, false
}

// Sort sorts the snapshots by version
func (mdbs *MultiDBSnapshots) Sort() {
	sort.Slice(mdbs.List, func(i, k int) bool {
		return mdbs.List[i].Version < mdbs.List[k].Version
	})
}

// LoadSnapshots calls LoadSnapshot on each added snapshot
func (mdbs *MultiDBSnapshots) LoadSnapshots() error {
	mdbs.Sort()
	for _, multiDBSnapshot := range mdbs.List {
		err := multiDBSnapshot.LoadSnapshot()
		if err != nil {
			return err
		}
	}
	return nil
}

// MultiDBSnapshot represents an expected state among multiple databases
type MultiDBSnapshot struct {
	Version   int
	Databases Databases
}

// LoadSnapshot parses the SQL and NewData fields on each database and loads the
// expected shema and data
func (mdbs *MultiDBSnapshot) LoadSnapshot() error {
	for _, database := range mdbs.Databases {
		snapshot, err := sqliteutil.LoadSnapshotFromSQL(fmt.Sprintf("%s\n%s", database.SQL, database.NewData))
		if err != nil {
			return err
		}
		database.Schema = snapshot.Schema
		database.Data = snapshot.Data
	}
	return nil
}

// Databases is a convenience type
type Databases map[string]*DBSnapshot

// DBSnapshot is a snapshot of a single DB. It separates the SQL and NewData,
// and after LoadSnapshot is called provides the shema and data.
type DBSnapshot struct {
	SQL     string
	NewData string

	// These are populated by the LoadSnapshot method
	Schema *dbschema.Schema
	Data   *dbschema.Data
}
