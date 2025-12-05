// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package migration

// MigrationTypes is a list of all possible migration types.
var MigrationTypes = []string{FullMigration, SnapshotMigration, TestDataCreation, NoMigration}

const (
	// FullMigration is a migration that migrates all data.
	FullMigration = "full"

	// SnapshotMigration is a migration that uses the latest database snapshot, instead of replaying all the steps.
	SnapshotMigration = "snapshot"

	// TestDataCreation is a migration that creates test data (in additional to full or snapshot).
	TestDataCreation = "testdata"

	// NoMigration is a migration that does not migrate any data.
	NoMigration = "none"
)
