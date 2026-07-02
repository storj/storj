// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package migration

import "strings"

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

// ShouldValidateVersion reports whether the database schema version should be
// validated after applying the given (comma separated) migration types.
//
// Validation is skipped when a full or snapshot migration was applied, since
// those bring the schema to a known state. For the no-migration case (the
// production default, where migrations are applied separately) the version is
// validated so the process fails fast against a mismatched schema.
func ShouldValidateVersion(migrationType string) bool {
	for t := range strings.SplitSeq(migrationType, ",") {
		switch strings.TrimSpace(t) {
		case FullMigration, SnapshotMigration:
			return false
		}
	}
	return true
}
