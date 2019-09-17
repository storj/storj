// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"
)

// MigrateTablesToDatabase copies the specified tables from srcDB into destDB.
// All tables in destDB will be dropped other than those specified in
// tablesToKeep.
func MigrateTablesToDatabase(ctx context.Context, srcDB, destDB *sql.DB, tablesToKeep ...string) error {
	// Retrieve the raw Sqlite3 driver connections for the src and dest so that
	// we can execute the backup API for a corruption safe clone.
	srcConn, err := srcDB.Conn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	destConn, err := destDB.Conn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	// The references to the driver connections are only guaranteed to be valid
	// for the life of the callback so we must do the work within both callbacks.
	err = srcConn.Raw(func(srcDriverConn interface{}) error {
		srcSqliteConn, ok := srcDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return errs.New("unable to get database driver")
		}

		err = destConn.Raw(func(destDriverConn interface{}) error {
			destSqliteConn, ok := destDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				return errs.New("unable to get database driver")
			}

			err = backup(ctx, srcSqliteConn, destSqliteConn)
			if err != nil {
				return errs.New("unable to backup database")
			}
			return nil
		})
		if err != nil {
			return errs.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(err)
	}

	if err := srcConn.Close(); err != nil {
		return errs.Wrap(err)
	}
	if err := destConn.Close(); err != nil {
		return errs.Wrap(err)
	}

	// Remove tables we don't want to keep from the cloned destination database.
	err = KeepTables(ctx, destDB, tablesToKeep...)
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// backup executes the sqlite3 backup process that safely ensures that no other
// connections to the database accidentally corrupt the source or destination.
func backup(ctx context.Context, sourceDB *sqlite3.SQLiteConn, destDB *sqlite3.SQLiteConn) error {
	// "main" represents the main (ie not "temp") database in sqlite3, which is
	// the database we want to backup, and the appropriate dest in the destDB
	backup, err := destDB.Backup("main", sourceDB, "main")
	if err != nil {
		return errs.Wrap(err)
	}

	isDone, err := backup.Step(0)
	if err != nil {
		return errs.Wrap(err)
	}
	if isDone {
		return errs.New("Backup is done")
	}

	// Check that the page count and remaining values are reasonable.
	initialPageCount := backup.PageCount()
	if initialPageCount <= 0 {
		return errs.New("initialPageCount invalid")
	}
	initialRemaining := backup.Remaining()
	if initialRemaining <= 0 {
		return errs.New("initialRemaining invalid")
	}
	if initialRemaining != initialPageCount {
		return errs.New("initialRemaining != initialPageCount")
	}

	// Step -1 is used to copy the entire source database to the destination.
	isDone, err = backup.Step(-1)
	if err != nil {
		return errs.Wrap(err)
	}
	if !isDone {
		return errs.New("Backup not done")
	}

	// Check that the page count and remaining values are reasonable.
	finalPageCount := backup.PageCount()
	if finalPageCount != initialPageCount {
		return errs.New("finalPageCount != initialPageCount")
	}
	finalRemaining := backup.Remaining()
	if finalRemaining != 0 {
		return errs.New("finalRemaining invalid")
	}

	// Finish the backup.
	err = backup.Finish()
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// KeepTables drops all the tables except the specified tables to keep.
func KeepTables(ctx context.Context, db *sql.DB, tablesToKeep ...string) error {
	// Get a list of tables excluding sqlite3 system tables.
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type ='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return errs.Wrap(err)
	}

	// Collect a list of the tables. We must do this because we can't do DDL
	// statements like drop tables while a query result is open.
	var tables []string
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			return errs.Combine(err, rows.Close())
		}
		tables = append(tables, tableName)
	}
	err = rows.Close()
	if err != nil {
		return errs.Wrap(err)
	}

	// Loop over the list of tables and decide which ones to keep and which to drop.
	for _, tableName := range tables {
		if !tableToKeep(tableName, tablesToKeep) {
			// Drop tables we aren't told to keep in the destination database.
			_, err = db.Exec(fmt.Sprintf("DROP TABLE %s;", tableName))
			if err != nil {
				return errs.Wrap(err)
			}
		}
	}

	// VACUUM the database to reclaim the space used by the dropped tables. The
	// data will not actually be reclaimed until the db has been closed.
	_, err = db.Exec("VACUUM;")
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func tableToKeep(table string, tables []string) bool {
	for _, t := range tables {
		if t == table {
			return true
		}
	}
	return false
}
