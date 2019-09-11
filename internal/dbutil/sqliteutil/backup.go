// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"
)

// ErrSqlite3Backup is the default error class for sqlite3_backup.
var ErrSqlite3Backup = errs.Class("sqlite3_backup")

// Sqlite3DriverName represents the custom Sqlite3 driver name.
const Sqlite3DriverName = "sqlite3_custom_"

// MigrateToDatabase backs up the specified Sqlite3 database and drops all tables not specified to keep in the destination database.
func MigrateToDatabase(ctx context.Context, connections map[string]*sqlite3.SQLiteConn, sqliteDriverInstanceKey string, sourceFileName string, destinationFileName string, tablesToKeep ...string) (err error) {
	sourceConn := connections[sourceFileName]
	sourceDir := filepath.Dir(sourceConn.GetFilename(""))
	destinationPath := filepath.Join(sourceDir, destinationFileName)

	// Check if the destination database is already opened. It should be because
	// we simplified the connection logic by opening all the databases on startup.
	// But if we are migrating those are all empty so to simplify the connection logic
	// we encapsulate all the complications here.
	// If the databaes exists we close and delete it. This keeps the Database object simple.
	if _, err := os.Stat(destinationPath); err == nil {
		destinationConn := connections[destinationFileName]
		err := destinationConn.Close()
		if err != nil {
			return ErrSqlite3Backup.Wrap(err)
		}
		err = os.Remove(destinationPath)
		if err != nil {
			return ErrSqlite3Backup.Wrap(err)
		}
	}

	destinationDB, err := sql.Open(sqliteDriverInstanceKey, "file:"+destinationPath+"?_journal=WAL&_busy_timeout=10000")
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Required to start the sqlite3 backup process.
	err = destinationDB.Ping()
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Fetch the Sqlite3 connection after the database has opened
	// and the connection hook has been called.
	destinationDatabase := connections[destinationFileName]

	// Execute the Sqlite3 backup process.
	err = backup(ctx, sourceConn, destinationDatabase)
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Get a list of tables excluding sqlite3 system tables.
	rows, err := destinationDB.Query("SELECT name FROM sqlite_master WHERE type ='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Collect a list of the tables. We must do this because we can't do DDL statements
	// like drop tables while a query result is open.
	tables := make([]string, 0)
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			return ErrSqlite3Backup.Wrap(err)
		}
		tables = append(tables, tableName)
	}
	err = rows.Close()
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Loop over the list of tables and decide which ones to keep and which to drop.
	for _, tableName := range tables {
		keepTable := false
		for _, table := range tablesToKeep {
			if strings.ToLower(tableName) == strings.ToLower(table) {
				keepTable = true
				break
			}
		}

		if keepTable == false {
			// Drop tables we aren't told to keep in the destination database.
			_, err = destinationDatabase.Exec("DROP TABLE "+tableName+";", nil)
			if err != nil {
				return ErrSqlite3Backup.Wrap(err)
			}
		}
	}

	// VACUUM the database to reclaim the space used by the dropped tables.
	_, err = destinationDB.Exec("VACUUM;")
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}
	return nil
}

// backup executes the sqlite3 backup process that safely ensures that no other
// connections to the database accidentally corrupt the source or destination.
func backup(ctx context.Context, sourceDB *sqlite3.SQLiteConn, destinationDB *sqlite3.SQLiteConn) error {
	backup, err := destinationDB.Backup("main", sourceDB, "main")
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}

	isDone, err := backup.Step(0)
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}
	if isDone {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Check that the page count and remaining values are reasonable.
	initialPageCount := backup.PageCount()
	if initialPageCount <= 0 {
		return ErrSqlite3Backup.Wrap(err)
	}
	initialRemaining := backup.Remaining()
	if initialRemaining <= 0 {
		return ErrSqlite3Backup.Wrap(err)
	}
	if initialRemaining != initialPageCount {
		return ErrSqlite3Backup.Wrap(err)
	}

	isDone, err = backup.Step(-1)
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}
	if !isDone {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Check that the page count and remaining values are reasonable.
	finalPageCount := backup.PageCount()
	if finalPageCount != initialPageCount {
		return ErrSqlite3Backup.Wrap(err)
	}
	finalRemaining := backup.Remaining()
	if finalRemaining != 0 {
		return ErrSqlite3Backup.Wrap(err)
	}

	// Finish the backup.
	err = backup.Finish()
	if err != nil {
		return ErrSqlite3Backup.Wrap(err)
	}
	return nil
}
