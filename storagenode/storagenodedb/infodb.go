// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/internal/dbutil/utccheck"
)

// ErrInfo is the default error class for InfoDB
var ErrInfo = errs.Class("infodb")

// SQLDB defines interface that matches *sql.DB
// this is such that we can use utccheck.DB for the backend
//
// TODO: wrap the connector instead of *sql.DB
type SQLDB interface {
	Begin() (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Close() error
	Conn(ctx context.Context) (*sql.Conn, error)
	Driver() driver.Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Ping() error
	PingContext(ctx context.Context) error
	Prepare(query string) (*sql.Stmt, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
}

// InfoDB implements information database for piecestore.
type InfoDB struct {
	db          SQLDB
	bandwidthdb bandwidthdb
	pieceinfo   pieceinfo
	location    string
}

// newInfo creates or opens InfoDB at the specified path.
func newInfo(path string) (*InfoDB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open(sqliteutil.Sqlite3DriverName, "file:"+path+"?_journal=WAL&_busy_timeout=10000")
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	dbutil.Configure(db, mon)

	infoDb := &InfoDB{
		db:       db,
		location: path,
	}

	infoDb.pieceinfo = pieceinfo{InfoDB: infoDb}
	infoDb.bandwidthdb = bandwidthdb{InfoDB: infoDb}

	return infoDb, nil
}

// NewInfoTest creates a new inmemory InfoDB.
func NewInfoTest() (*InfoDB, error) {
	// create memory DB with a shared cache and a unique name to avoid collisions
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", rand.Int63()))
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	// Set max idle and max open to 1 to control concurrent access to the memory DB
	// Setting max open higher than 1 results in table locked errors
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(-1)

	mon.Chain("db_stats", monkit.StatSourceFunc(
		func(cb func(name string, val float64)) {
			monkit.StatSourceFromStruct(db.Stats()).Stats(cb)
		}))

	infoDb := &InfoDB{db: utccheck.New(db)}
	infoDb.pieceinfo = pieceinfo{InfoDB: infoDb}
	infoDb.bandwidthdb = bandwidthdb{InfoDB: infoDb}

	return infoDb, nil
}

// Close closes any resources.
func (db *InfoDB) Close() error {
	return db.db.Close()
}

// CreateTables creates any necessary tables.
// func (db *InfoDB) CreateTables(log *zap.Logger) error {
// 	migration := db.Migration()
// 	return migration.Run(log.Named("migration"), db)
// }

// RawDB returns access to the raw database, only for migration tests.
func (db *InfoDB) RawDB() SQLDB { return db.db }

// Begin begins transaction
func (db *InfoDB) Begin() (*sql.Tx, error) { return db.db.Begin() }

// Rebind rebind parameters
func (db *InfoDB) Rebind(s string) string { return s }

// Schema returns schema
func (db *InfoDB) Schema() string { return "" }
