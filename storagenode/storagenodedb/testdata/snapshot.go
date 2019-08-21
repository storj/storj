// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

// Snapshot represents a snapshot of the database schema and data
// for migration testing purposes.
type Snapshot struct {
	Version int
	Steps   []SnapshotStep
}

// SnapshotStep represents a step in a snapshot that executes
// against the specified database.
type SnapshotStep struct {
	Database   string
	Statements string
}

// GetSnapshots returns all the test data snapshots.
func GetSnapshots() []Snapshot {
	snapshots := []Snapshot{
		Snapshot_v0,
		Snapshot_v1,
	}

	return snapshots
}
