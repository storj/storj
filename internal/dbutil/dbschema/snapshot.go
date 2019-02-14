// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema

// Snapshots defines a collection of snapshot
type Snapshots struct {
	List []*Snapshot
}

// Snapshot defines a particular snapshot of schema and data.
type Snapshot struct {
	Version int
	Script  string
	*Schema
	*Data
}

// Add adds a new snapshot.
func (snapshots *Snapshots) Add(snap *Snapshot) {
	snapshots.List = append(snapshots.List, snap)
}

// FindVersion finds a snapshot with the specified version.
func (snapshots *Snapshots) FindVersion(version int) (*Snapshot, bool) {
	for _, snap := range snapshots.List {
		if snap.Version == version {
			return snap, true
		}
	}
	return nil, false
}
