// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/storagenode/storagenodedb"
)

// SetupArgs contains arguments for the setup command.
type SetupArgs struct {
}

// Setup handles storagenode configuration setup.
type Setup struct {
	dbConfig storagenodedb.Config
	confDir  cli.ConfigDir
	identity *identity.FullIdentity
}

// NewSetup creates a new Setup instance for storagenode configuration.
func NewSetup(dbConfig storagenodedb.Config, confDir cli.ConfigDir, identity *identity.FullIdentity) *Setup {
	return &Setup{
		dbConfig: dbConfig,
		confDir:  confDir,
		identity: identity,
	}
}

// Run executes the storagenode setup process.
func (s *Setup) Run(ctx context.Context) error {
	setupDir := s.confDir.Dir
	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("storagenode configuration already exists (%v)", setupDir)
	}

	// create db
	db, err := storagenodedb.OpenNew(ctx, zap.L().Named("db"), s.dbConfig)
	if err != nil {
		return err
	}

	if err := db.Pieces().CreateVerificationFile(ctx, s.identity.ID); err != nil {
		return err
	}

	return nil
}
