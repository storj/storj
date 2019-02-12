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
	Description string
	Version     int // Versions should start at 0
	Action      Action
}

// Action is something that needs to be done
type Action interface {
	Run(log *zap.Logger, db DB, tx *sql.Tx) error
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
func (migration *Migration) Run(log *zap.Logger, db DB) error {
	err := migration.ValidTableName()
	if err != nil {
		return err
	}

	err = migration.ValidateSteps()
	if err != nil {
		return err
	}

	err = migration.ensureVersionTable(log, db)
	if err != nil {
		return Error.New("creating version table failed: %v", err)
	}

	version, err := migration.getLatestVersion(log, db)
	if err != nil {
		return Error.Wrap(err)
	}

	if version >= 0 {
		log.Info("Latest Version", zap.Int("version", version))
	} else {
		log.Info("No Version")
	}

	for _, step := range migration.Steps {
		if step.Version <= version {
			continue
		}

		log := log.Named(strconv.Itoa(step.Version))
		log.Info(step.Description)

		tx, err := db.Begin()
		if err != nil {
			return Error.Wrap(err)
		}

		err = step.Action.Run(log, db, tx)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		err = migration.addVersion(tx, db, step.Version)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		if err := tx.Commit(); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// createVersionTable creates a new version table
func (migration *Migration) ensureVersionTable(log *zap.Logger, db DB) error {
	tx, err := db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Exec(db.Rebind(`CREATE TABLE IF NOT EXISTS ` + migration.Table + ` (version int, commited_at text)`))
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
	err = tx.QueryRow(db.Rebind(`SELECT MAX(version) FROM ` + migration.Table)).Scan(&version)
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
	_, err := tx.Exec(db.Rebind(`
		INSERT INTO `+migration.Table+` (version, commited_at)
		VALUES (?, ?)`),
		version, time.Now().String(),
	)
	return err
}

// SQL statements that are executed on the database
type SQL []string

// Run runs the SQL statements
func (sql SQL) Run(log *zap.Logger, db DB, tx *sql.Tx) (err error) {
	for _, query := range sql {
		_, err := tx.Exec(db.Rebind(query))
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
