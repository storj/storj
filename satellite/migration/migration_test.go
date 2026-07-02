// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package migration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/migration"
)

func TestShouldValidateVersion(t *testing.T) {
	for _, tt := range []struct {
		migrationType string
		validate      bool
	}{
		{migration.NoMigration, true},
		{"", true},
		{migration.TestDataCreation, true},
		{migration.NoMigration + "," + migration.TestDataCreation, true},
		{migration.FullMigration, false},
		{migration.SnapshotMigration, false},
		{migration.FullMigration + "," + migration.TestDataCreation, false},
		{migration.SnapshotMigration + "," + migration.TestDataCreation, false},
		{" full , testdata ", false},
	} {
		require.Equal(t, tt.validate, migration.ShouldValidateVersion(tt.migrationType), tt.migrationType)
	}
}
