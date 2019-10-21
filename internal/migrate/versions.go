// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"database/sql"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
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

// Migration describes a migration steps
type Migration struct {
	Table string
	Steps []*Step
}

// Step describes a single step in migration.
type Step struct {
	DB          DB // The DB to execute this step on
	Description string
	Version     int // Versions should start at 0
	Action      Action
}

// Action is something that needs to be done
type Action interface {
	Run(log *zap.Logger, db DB, tx *sql.Tx) error
}

// TargetVersion returns migration with steps upto specified version
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

// ValidTableName checks whether the specified table name is valid
func (migration *Migration) ValidTableName() error {
	matched, err := regexp.MatchString(`^[a-z_]+$`, migration.Table)
	if !matched || err != nil {
		return Error.New("invalid table name: %v", migration.Table)
	}
	return nil
}

// ValidateSteps checks whether the specified table name is valid
func (migration *Migration) ValidateSteps() error {
	sorted := sort.SliceIsSorted(migration.Steps, func(i, j int) bool {
		return migration.Steps[i].Version <= migration.Steps[j].Version
	})
	if !sorted {
		return Error.New("steps have incorrect order")
	}
	return nil
}

// Run runs the migration steps
func (migration *Migration) Run(log *zap.Logger) error {
	err := migration.ValidTableName()
	if err != nil {
		return err
	}

	err = migration.ValidateSteps()
	if err != nil {
		return err
	}

	for _, step := range migration.Steps {
		if step.DB == nil {
			return Error.New("step.DB is nil for step %d", step.Version)
		}

		err = migration.ensureVersionTable(log, step.DB)
		if err != nil {
			return Error.New("creating version table failed: %v", err)
		}

		version, err := migration.getLatestVersion(log, step.DB)
		if err != nil {
			return Error.Wrap(err)
		}

		if step.Version <= version {
			continue
		}

		stepLog := log.Named(strconv.Itoa(step.Version))
		stepLog.Info(step.Description)

		tx, err := step.DB.Begin()
		if err != nil {
			return Error.Wrap(err)
		}

		err = step.Action.Run(stepLog, step.DB, tx)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		err = migration.addVersion(tx, step.DB, step.Version)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		if err := tx.Commit(); err != nil {
			return Error.Wrap(err)
		}
	}

	if len(migration.Steps) > 0 {
		last := migration.Steps[len(migration.Steps)-1]
		log.Info("Database Version", zap.Int("version", last.Version))
	} else {
		log.Info("No Versions")
	}

	return nil
}

// createVersionTable creates a new version table
func (migration *Migration) ensureVersionTable(log *zap.Logger, db DB) error {
	tx, err := db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Exec(rebind(db, `CREATE TABLE IF NOT EXISTS `+migration.Table+` (version int, commited_at text)`)) //nolint:misspell
	if err != nil {
		return Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return Error.Wrap(tx.Commit())
}

// getLatestVersion finds the latest version table
func (migration *Migration) getLatestVersion(log *zap.Logger, db DB) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return -1, Error.Wrap(err)
	}

	var version sql.NullInt64
	err = tx.QueryRow(rebind(db, `SELECT MAX(version) FROM `+migration.Table)).Scan(&version)
	if err == sql.ErrNoRows || !version.Valid {
		return -1, Error.Wrap(tx.Commit())
	}
	if err != nil {
		return -1, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return int(version.Int64), Error.Wrap(tx.Commit())
}

// addVersion adds information about a new migration
func (migration *Migration) addVersion(tx *sql.Tx, db DB, version int) error {
	_, err := tx.Exec(rebind(db, `
		INSERT INTO `+migration.Table+` (version, commited_at) VALUES (?, ?)`), //nolint:misspell
		version, time.Now().String(),
	)
	return err
}

// SQL statements that are executed on the database
type SQL []string

// Run runs the SQL statements
func (sql SQL) Run(log *zap.Logger, db DB, tx *sql.Tx) (err error) {
	for _, query := range sql {
		_, err := tx.Exec(rebind(db, query))
		if err != nil {
			return err
		}
	}
	return nil
}

// Func is an arbitrary operation
type Func func(log *zap.Logger, db DB, tx *sql.Tx) error

// Run runs the migration
func (fn Func) Run(log *zap.Logger, db DB, tx *sql.Tx) error {
	return fn(log, db, tx)
}
