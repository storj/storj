// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

var (
	// ErrValidateVersionQuery is when there is an error querying version table.
	ErrValidateVersionQuery = errs.Class("validate db version query")
	// ErrValidateVersionMismatch is when the migration version does not match the current database version.
	ErrValidateVersionMismatch = errs.Class("validate db version mismatch")
	// ErrValidateMinVersion is when the migration version does not match the current database version.
	ErrValidateMinVersion = errs.Class("validate minimum version")
)

/*

Scenarios it doesn't handle properly.

1. Rollback to initial state on multi-step migration.

	Let's say there's a scenario where we run migration steps:
	1. update a table schema
	2. move files
	3. update a table schema
	4. update a table schema, which fails

	In this case there's no easy way to rollback the moving of files.

2. Undoing migrations.

	Intentionally left out, because we do not gain that much from currently.

3. Snapshotting the whole state.

	This probably should be done by the user of this library, when there's disk-space available.

4. Figuring out what the exact executed steps are.
*/

// Migration describes a migration steps.
type Migration struct {
	// Table is the table name to register the applied migration version.
	// NOTE: Always validates its value with the ValidTableName method before it's
	// concatenated in a query string for avoiding SQL injection attacks.
	Table string
	Steps []*Step
}

// Step describes a single step in migration.
type Step struct {
	DB          *tagsql.DB // The DB to execute this step on
	Description string
	Version     int // Versions should start at 0
	Action      Action
	CreateDB    CreateDB

	// SeparateTx marks a step as it should not be merged together for optimization.
	// Cockroach cannot add a column and update the value in the same transaction.
	SeparateTx bool
}

// Action is something that needs to be done.
type Action interface {
	Run(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error
}

// TargetVersion returns migration with steps upto specified version.
func (migration *Migration) TargetVersion(version int) *Migration {
	m := *migration
	m.Steps = nil
	for _, step := range migration.Steps {
		if step.Version <= version {
			m.Steps = append(m.Steps, step)
		}
	}
	return &m
}

// ValidTableName checks whether the specified table name is only formed by at
// least one character and its only formed by lowercase letters and underscores.
//
// NOTE: if you change this function to accept a wider range of characters, make
// sure that they cannot open to SQL injections because Table field is used
// concatenated in some queries performed by Mitration methods.
func (migration *Migration) ValidTableName() error {
	matched, err := regexp.MatchString(`^[a-z_]+$`, migration.Table)
	if !matched || err != nil {
		return Error.New("invalid table name: %v", migration.Table)
	}
	return nil
}

// ValidateSteps checks that the version for each migration step increments in order.
func (migration *Migration) ValidateSteps() error {
	sorted := sort.SliceIsSorted(migration.Steps, func(i, j int) bool {
		return migration.Steps[i].Version <= migration.Steps[j].Version
	})
	if !sorted {
		return Error.New("steps have incorrect order")
	}
	return nil
}

// ValidateVersions checks that the version of the migration matches the state of the database.
func (migration *Migration) ValidateVersions(ctx context.Context, log *zap.Logger) error {
	if err := migration.ValidateSteps(); err != nil {
		return err
	}

	expectedVersions := make(map[tagsql.DB]int)
	for _, step := range migration.Steps {
		expectedVersions[*step.DB] = step.Version
	}

	for database, expectedVersion := range expectedVersions {
		currentVersion, err := migration.CurrentVersion(ctx, log, database)
		if err != nil {
			return ErrValidateVersionQuery.Wrap(err)
		}

		if expectedVersion != currentVersion {
			if currentVersion < 0 {
				return ErrValidateVersionMismatch.New("expected %d, but database is uninitialized (version %d)", expectedVersion, currentVersion)
			}
			return ErrValidateVersionMismatch.New("expected %d, but current version is %d", expectedVersion, currentVersion)
		}
	}

	if len(migration.Steps) > 0 {
		last := migration.Steps[len(migration.Steps)-1]
		log.Debug("Database version is up to date", zap.Int("version", last.Version))
	} else {
		log.Debug("No Versions")
	}

	return nil
}

// Run runs the migration steps.
func (migration *Migration) Run(ctx context.Context, log *zap.Logger) error {
	err := migration.ValidateSteps()
	if err != nil {
		return err
	}

	initialSetup := false
	for i, step := range migration.Steps {
		step := step

		if step.CreateDB != nil {
			if err := step.CreateDB(ctx, log); err != nil {
				return Error.Wrap(err)
			}
		}

		db := *step.DB
		if db == nil {
			return Error.New("step.DB is nil for step %d", step.Version)
		}

		err = migration.ensureVersionTable(ctx, log, db)
		if err != nil {
			return Error.New("creating version table failed: %w", err)
		}

		version, err := migration.getLatestVersion(ctx, log, db)
		if err != nil {
			return Error.Wrap(err)
		}
		if i == 0 && version < 0 {
			initialSetup = true
		}

		if step.Version <= version {
			continue
		}

		stepLog := log.Named(strconv.Itoa(step.Version))
		if !initialSetup {
			stepLog.Info(step.Description)
		}

		err = txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) error {
			err = step.Action.Run(ctx, stepLog, db, tx)
			if err != nil {
				return err
			}

			err = migration.addVersion(ctx, tx, db, step.Version)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return Error.New("v%d: %w", step.Version, err)
		}
	}

	if len(migration.Steps) > 0 {
		last := migration.Steps[len(migration.Steps)-1]
		if initialSetup {
			log.Info("Database Created", zap.Int("version", last.Version))
		} else {
			log.Info("Database Version", zap.Int("version", last.Version))
		}
	} else {
		log.Info("No Versions")
	}

	return nil
}

// ensureVersionTable creates migration.Table table if not exists.
func (migration *Migration) ensureVersionTable(ctx context.Context, log *zap.Logger, db tagsql.DB) error {
	err := txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		_, err := tx.Exec(ctx, rebind(db, `CREATE TABLE IF NOT EXISTS `+migration.Table+` (version int, commited_at text)`)) //nolint:misspell
		return err
	})
	return Error.Wrap(err)
}

// getLatestVersion finds the latest version in migration.Table.
// It returns -1 if there aren't rows or version is null.
func (migration *Migration) getLatestVersion(ctx context.Context, log *zap.Logger, db tagsql.DB) (int, error) {
	err := migration.ValidTableName()
	if err != nil {
		return 0, err
	}

	var version sql.NullInt64
	err = txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		/* #nosec G202 */ // Table name is white listed by the ValidTableName method
		// executed at the beginning of the function
		err := tx.QueryRow(ctx, rebind(db, `SELECT MAX(version) FROM `+migration.Table)).Scan(&version)
		if errors.Is(err, sql.ErrNoRows) || !version.Valid {
			version.Int64 = -1
			return nil
		}
		return err
	})

	return int(version.Int64), Error.Wrap(err)
}

// addVersion adds information about a new migration.
func (migration *Migration) addVersion(ctx context.Context, tx tagsql.Tx, db tagsql.DB, version int) error {
	err := migration.ValidTableName()
	if err != nil {
		return err
	}

	/* #nosec G202 */ // Table name is white listed by the ValidTableName method
	// executed at the beginning of the function
	_, err = tx.Exec(ctx, rebind(db, `
		INSERT INTO `+migration.Table+` (version, commited_at) VALUES (?, ?)`), //nolint:misspell
		version, time.Now().String(),
	)
	return err
}

// CurrentVersion finds the latest version for the db.
func (migration *Migration) CurrentVersion(ctx context.Context, log *zap.Logger, db tagsql.DB) (int, error) {
	err := migration.ensureVersionTable(ctx, log, db)
	if err != nil {
		return -1, Error.Wrap(err)
	}
	return migration.getLatestVersion(ctx, log, db)
}

// SQL statements that are executed on the database.
type SQL []string

// Run runs the SQL statements.
func (sql SQL) Run(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) (err error) {
	for _, query := range sql {
		_, err := tx.Exec(ctx, rebind(db, query))
		if err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}

// Func is an arbitrary operation.
type Func func(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error

// Run runs the migration.
func (fn Func) Run(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error {
	return fn(ctx, log, db, tx)
}

// CreateDB is operation for creating new dbs.
type CreateDB func(ctx context.Context, log *zap.Logger) error
