// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// MigrationTargetTier represents the target pricing tier for project migration.
type MigrationTargetTier string

const (
	// MigrationTargetTierArchive migrates legacy placements to the archive tier.
	MigrationTargetTierArchive MigrationTargetTier = "archive"
	// MigrationTargetTierGlobal migrates legacy placements to the new global tier.
	MigrationTargetTierGlobal MigrationTargetTier = "global"
)

var validMigrationTargetTiers = map[MigrationTargetTier]struct{}{
	MigrationTargetTierArchive: {},
	MigrationTargetTierGlobal:  {},
}

// IsValid validates MigrationTargetTier value.
func (mtt MigrationTargetTier) IsValid() bool {
	_, ok := validMigrationTargetTiers[mtt]
	return ok
}
