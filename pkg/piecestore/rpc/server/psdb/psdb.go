// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // register sqlite to sql

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/utils"
)

var (
	mon                  = monkit.Package()
	defaultCheckInterval = flag.Duration("piecestore.ttl.check_interval", time.Hour, "number of seconds to sleep between ttl checks")
)

// DB is a piece store database
type DB struct {
	dataPath string
	mu       sync.Mutex
	DB       *sql.DB // TODO: hide
	check    *time.Ticker
}

// Open opens DB at DBPath
func Open(ctx context.Context, DataPath, DBPath string) (db *DB, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = os.MkdirAll(filepath.Dir(DBPath), 0700); err != nil {
		return nil, err
	}

	sqlite, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&mutex=full", DBPath))
	if err != nil {
		return nil, err
	}

	// try to enable write-ahead-logging
	_, _ = sqlite.Exec(`PRAGMA journal_mode = WAL`)

	defer func() {
		if err != nil {
			_ = sqlite.Close()
		}
	}()

	tx, err := sqlite.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`id` BLOB UNIQUE, `created` INT(10), `expires` INT(10), `size` INT(10));")
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `bandwidth_agreements` (`agreement` BLOB, `signature` BLOB);")
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_ttl_expires ON ttl (expires);")
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `bwusagetbl` (`size` INT(10), `daystartdate` INT(10), `dayenddate` INT(10));")
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	db = &DB{
		DB:       sqlite,
		dataPath: DataPath,
		check:    time.NewTicker(*defaultCheckInterval),
	}
	go db.garbageCollect(ctx)

	return db, nil
}

// Close the database
func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) locked() func() {
	db.mu.Lock()
	return db.mu.Unlock
}

// DeleteExpired checks for expired TTLs in the DB and removes data from both the DB and the FS
func (db *DB) DeleteExpired(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var expired []string
	err = func() error {
		defer db.locked()()

		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()

		now := time.Now().Unix()

		rows, err := tx.Query("SELECT id FROM ttl WHERE 0 < expires AND ? < expires", now)
		if err != nil {
			return err
		}

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			expired = append(expired, id)
		}
		if err := rows.Close(); err != nil {
			return err
		}

		_, err = tx.Exec(`DELETE FROM ttl WHERE 0 < expires AND ? < expires`, now)
		if err != nil {
			return err
		}

		return tx.Commit()
	}()

	var errs []error
	for _, id := range expired {
		err := pstore.Delete(id, db.dataPath)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return utils.CombineErrors(errs...)
	}

	return nil
}

// garbageCollect will periodically run DeleteExpired
func (db *DB) garbageCollect(ctx context.Context) {
	for range db.check.C {
		err := db.DeleteExpired(ctx)
		if err != nil {
			zap.S().Errorf("failed checking entries: %+v", err)
		}
	}
}

// WriteBandwidthAllocToDB -- Insert bandwidth agreement into DB
func (db *DB) WriteBandwidthAllocToDB(ba *pb.RenterBandwidthAllocation) error {
	defer db.locked()()

	_, err := db.DB.Exec(`INSERT INTO bandwidth_agreements (agreement, signature) VALUES (?, ?)`, ba.GetData(), ba.GetSignature())
	return err
}

// GetBandwidthAllocationBySignature finds allocation info by signature
func (db *DB) GetBandwidthAllocationBySignature(signature []byte) ([][]byte, error) {
	defer db.locked()()

	rows, err := db.DB.Query(`SELECT agreement FROM bandwidth_agreements WHERE signature = ?`, signature)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	agreements := [][]byte{}
	for rows.Next() {
		var agreement []byte
		err := rows.Scan(&agreement)
		if err != nil {
			return agreements, err
		}
		agreements = append(agreements, agreement)
	}
	return agreements, nil
}

// AddTTL adds TTL into database by id
func (db *DB) AddTTL(id string, expiration, size int64) error {
	defer db.locked()()

	created := time.Now().Unix()
	_, err := db.DB.Exec("INSERT OR REPLACE INTO ttl (id, created, expires, size) VALUES (?, ?, ?, ?)", id, created, expiration, size)
	return err
}

// GetTTLByID finds the TTL in the database by id and return it
func (db *DB) GetTTLByID(id string) (expiration int64, err error) {
	defer db.locked()()

	err = db.DB.QueryRow(`SELECT expires FROM ttl WHERE id=?`, id).Scan(&expiration)
	return expiration, err
}

// SumTTLSizes sums the size column on the ttl table
func (db *DB) SumTTLSizes() (sum int64, err error) {
	defer db.locked()()

	var count int
	rows := db.DB.QueryRow("SELECT COUNT(*) as count FROM ttl")
	err = rows.Scan(&count)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	err = db.DB.QueryRow(`SELECT SUM(size) FROM ttl;`).Scan(&sum)
	return sum, err
}

// DeleteTTLByID finds the TTL in the database by id and delete it
func (db *DB) DeleteTTLByID(id string) error {
	defer db.locked()()

	_, err := db.DB.Exec(`DELETE FROM ttl WHERE id=?`, id)
	if err == sql.ErrNoRows {
		err = nil
	}
	return err
}

// AddBandwidthUsed adds bandwidth usage into database by date
func (db *DB) AddBandwidthUsed(size int64) (err error) {
	defer db.locked()()

	t := time.Now()
	daystartunixtime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	dayendunixtime := time.Date(t.Year(), t.Month(), t.Day(), 24, 0, 0, 0, t.Location()).Unix()

	var getSize int64
	if (t.Unix() >= daystartunixtime) && (t.Unix() <= dayendunixtime) {
		err = db.DB.QueryRow(`SELECT size FROM bwusagetbl WHERE daystartdate <= ? AND ? <= dayenddate`, t.Unix(), t.Unix()).Scan(&getSize)
		switch {
		case err == sql.ErrNoRows:
			_, err = db.DB.Exec("INSERT INTO bwusagetbl (size, daystartdate, dayenddate) VALUES (?, ?, ?)", size, daystartunixtime, dayendunixtime)
			return err
		case err != nil:
			return err
		default:
			getSize = size + getSize
			_, err = db.DB.Exec("UPDATE bwusagetbl SET size = ? WHERE daystartdate = ?", getSize, daystartunixtime)
			return err
		}
	}
	return err
}

// GetBandwidthUsedByDay finds the so far bw used by day and return it
func (db *DB) GetBandwidthUsedByDay(t time.Time) (size int64, err error) {
	defer db.locked()()

	daystarttime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	err = db.DB.QueryRow(`SELECT size FROM bwusagetbl WHERE daystartdate=?`, daystarttime).Scan(&size)
	return size, err
}

// GetTotalBandwidthBetween each row in the bwusagetbl contains the total bw used per day
func (db *DB) GetTotalBandwidthBetween(startdate time.Time, enddate time.Time) (totalbwusage int64, err error) {
	defer db.locked()()

	startTimeUnix := time.Date(startdate.Year(), startdate.Month(), startdate.Day(), 0, 0, 0, 0, startdate.Location()).Unix()
	endTimeUnix := time.Date(enddate.Year(), enddate.Month(), enddate.Day(), 0, 0, 0, 0, enddate.Location()).Unix()
	defaultunixtime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location()).Unix()

	if (endTimeUnix < startTimeUnix) && (startTimeUnix > defaultunixtime || endTimeUnix > defaultunixtime) {
		return totalbwusage, errors.New("Invalid date range")
	}

	err = db.DB.QueryRow(`SELECT SUM(size) FROM bwusagetbl WHERE daystartdate BETWEEN ? AND ?`, startTimeUnix, endTimeUnix).Scan(&totalbwusage)
	return totalbwusage, err
}
