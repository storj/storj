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

// ErrSqlite3Migrator is the default error class for sqlite3_migrator.
var ErrSqlite3Migrator = errs.Class("sqlite3_migrator")

type Migrator struct {
	dbs map[string]*sql.DB
}

func NewMigrator(dbs map[string]*sql.DB) *Migrator {
	return &Migrator{
		dbs: dbs,
	}
}

func (m *Migrator) MigrateTablesToDatabase(ctx context.Context, srcFilename string, destFilename string, tablesToKeep ...string) error {
	srcDB, found := m.dbs[srcFilename]
	if !found {
		return ErrSqlite3Migrator.New("unable to get database for %s", srcFilename)
	}

	destDB, found := m.dbs[destFilename]
	if !found {
		return ErrSqlite3Migrator.New("unable to get database for %s", destFilename)
	}
	destPath, err := m.getFilepathForDatabase(ctx, destDB)
	if err != nil {
		return err
	}

	// We clean up the destination database because we've already opened it and it'll be empty
	// during a migration. We've done this to simplify the connection logic within the common running code
	// and placed the complexity in the one time migration code.
	m.deleteDatabase(destDB, destPath)

	// Create the new destination database.
	destDB, err = sql.Open("sqlite3", "file:"+destPath+"?_journal=WAL&_busy_timeout=10000")
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	m.dbs[destFilename] = destDB

	// Now we retrieve the raw Sqlite3 driver connections for the src and dest
	// so that we can execute the backup API for a corruption safe clone.
	srcConn, err := srcDB.Conn(ctx)
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	destConn, err := destDB.Conn(ctx)
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}

	// The references to the driver connections are only guaranteed to be valid
	// for the life of the callback so we must do the work within both callbacks.
	srcConn.Raw(func(srcDriverConn interface{}) error {
		if srcSqliteConn, ok := srcDriverConn.(*sqlite3.SQLiteConn); ok {
			destConn.Raw(func(destDriverConn interface{}) error {
				if destSqliteConn, ok := destDriverConn.(*sqlite3.SQLiteConn); ok {
					err = m.backup(ctx, srcSqliteConn, destSqliteConn)
					if err != nil {
						return ErrSqlite3Migrator.New("unable to backup database")
					}
				} else {
					return ErrSqlite3Migrator.New("unable to get database driver")
				}
				return nil
			})

		} else {
			return ErrSqlite3Migrator.New("unable to get database driver")
		}
		return nil
	})

	// Remove tables we don't want to keep from the cloned destination database.
	err = m.KeepTables(ctx, destDB, tablesToKeep...)
	if err != nil {
		return ErrSqlite3Migrator.New("unable to get database driver")
	}

	return nil
}

func (m *Migrator) getFilepathForDatabase(ctx context.Context, db *sql.DB) (filepath string, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return filepath, ErrSqlite3Migrator.New("unable to get database connection")
	}

	err = conn.Raw(func(driverConn interface{}) error {
		if sqliteConn, ok := driverConn.(*sqlite3.SQLiteConn); ok {
			filepath = sqliteConn.GetFilename("")
		} else {
			return ErrSqlite3Migrator.New("unable to get database driver")
		}
		return nil
	})
	if err != nil {
		return filepath, err
	}

	return filepath, nil
}

// cleanup closes the specified database and deletes the database file on disk.
func (m *Migrator) deleteDatabase(db *sql.DB, filePath string) error {
	if err := db.Close(); err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	if _, err := os.Stat(filePath); err == nil {
		err = os.Remove(filePath)
		if err != nil {
			return ErrSqlite3Migrator.Wrap(err)
		}
	}
	filename := filepath.Base(filePath)
	delete(m.dbs, filename)
	return nil
}

// backup executes the sqlite3 backup process that safely ensures that no other
// connections to the database accidentally corrupt the source or destination.
func (m *Migrator) backup(ctx context.Context, sourceDB *sqlite3.SQLiteConn, destDB *sqlite3.SQLiteConn) error {
	backup, err := destDB.Backup("main", sourceDB, "main")
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}

	isDone, err := backup.Step(0)
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	if isDone {
		return ErrSqlite3Migrator.Wrap(err)
	}

	// Check that the page count and remaining values are reasonable.
	initialPageCount := backup.PageCount()
	if initialPageCount <= 0 {
		return ErrSqlite3Migrator.Wrap(err)
	}
	initialRemaining := backup.Remaining()
	if initialRemaining <= 0 {
		return ErrSqlite3Migrator.Wrap(err)
	}
	if initialRemaining != initialPageCount {
		return ErrSqlite3Migrator.Wrap(err)
	}

	isDone, err = backup.Step(-1)
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	if !isDone {
		return ErrSqlite3Migrator.Wrap(err)
	}

	// Check that the page count and remaining values are reasonable.
	finalPageCount := backup.PageCount()
	if finalPageCount != initialPageCount {
		return ErrSqlite3Migrator.Wrap(err)
	}
	finalRemaining := backup.Remaining()
	if finalRemaining != 0 {
		return ErrSqlite3Migrator.Wrap(err)
	}

	// Finish the backup.
	err = backup.Finish()
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	return nil
}

// keepTables drops all the tables except the specified tables to keep.
func (m *Migrator) KeepTables(ctx context.Context, db *sql.DB, tablesToKeep ...string) error {
	// Get a list of tables excluding sqlite3 system tables.
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type ='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}

	// Collect a list of the tables. We must do this because we can't do DDL statements
	// like drop tables while a query result is open.
	tables := make([]string, 0)
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			return ErrSqlite3Migrator.Wrap(err)
		}
		tables = append(tables, tableName)
	}
	err = rows.Close()
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
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
			_, err = db.Exec("DROP TABLE "+tableName+";", nil)
			if err != nil {
				return ErrSqlite3Migrator.Wrap(err)
			}
		}
	}

	// VACUUM the database to reclaim the space used by the dropped tables.
	_, err = db.Exec("VACUUM;")
	if err != nil {
		return ErrSqlite3Migrator.Wrap(err)
	}
	return nil
}
