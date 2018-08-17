// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	_ "github.com/mattn/go-sqlite3" // sqlite is weird and needs underscore

	"storj.io/storj/pkg/piecestore"
	pb "storj.io/storj/protos/piecestore"
)

var (
	mon = monkit.Package()
)

// PSDB -- Piecestore database
type PSDB struct {
	DB            *sql.DB
	mtx           sync.Mutex
	dataPath      string
	checkInterval time.Duration
}

// OpenPSDB -- opens PSDB at DBPath
func OpenPSDB(ctx context.Context, DataPath, DBPath string) (psdb *PSDB, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = os.MkdirAll(filepath.Dir(DBPath), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&mutex=full", DBPath))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			db.Close()
		}
	}()

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	defer rollback(tx)

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`id` TEXT UNIQUE, `created` INT(10), `expires` INT(10));")
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `bandwidth_agreements` (`agreement` BLOB, `signature` BLOB);")
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &PSDB{DB: db, dataPath: DataPath, checkInterval: 300}, nil
}

// deleteEntries -- checks for and deletes expired TTL entries
func deleteEntriesFromFS(dir string, rows *sql.Rows) error {
	for rows.Next() {
		var expID string

		err := rows.Scan(&expID)
		if err != nil {
			return err
		}

		// delete file on local machine
		err = pstore.Delete(expID, dir)
		if err != nil {
			return err
		}

		log.Printf("Deleted file: %s\n", expID)
	}

	return rows.Err()
}

// Close the database
func (psdb *PSDB) Close() error {
	return psdb.DB.Close()
}

// DeleteExpired checks for expired TTLs in the DB and removes data from both the DB and the FS
func (psdb *PSDB) DeleteExpired(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().Unix()

	rows, err := psdb.DB.Query(fmt.Sprintf("SELECT id FROM ttl WHERE expires < %d AND expires > 0", now))
	if err != nil {
		return err
	}

	defer func() {
		if rowErr := rows.Close(); rowErr != nil {
			log.Printf("failed to close Rows: %s\n", rowErr)
		}
	}()

	if err = deleteEntriesFromFS(psdb.dataPath, rows); err != nil {
		return err
	}

	_, err = psdb.DB.Exec(fmt.Sprintf("DELETE FROM ttl WHERE expires < %d AND expires > 0", now))
	if err != nil {
		return err
	}

	return nil
}

// DeleteExpiredLoop will periodically run DeleteExpired
func (psdb *PSDB) DeleteExpiredLoop(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(psdb.checkInterval)
			err = ctx.Err()
			if err != nil {
				return err
			}
			err = psdb.DeleteExpired(ctx)
			if err != nil {
				zap.S().Errorf("failed checking entries: %+v", err)
			}
		}
	}
}

// WriteBandwidthAllocToDB -- Insert bandwidth agreement into DB
func (psdb *PSDB) WriteBandwidthAllocToDB(ba *pb.RenterBandwidthAllocation) error {
	psdb.mtx.Lock()
	defer psdb.mtx.Unlock()

	data := ba.GetData()
	if data == nil {
		return nil
	}

	tx, err := psdb.DB.Begin()
	if err != nil {
		return err
	}

	defer rollback(tx)

	stmt, err := psdb.DB.Prepare(`INSERT INTO bandwidth_agreements (agreement, signature) VALUES (?, ?)`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = tx.Stmt(stmt).Exec(data, ba.GetSignature())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// AddTTLToDB -- Insert TTL into database by id
func (psdb *PSDB) AddTTLToDB(id string, expiration int64) error {
	psdb.mtx.Lock()
	defer psdb.mtx.Unlock()

	tx, err := psdb.DB.Begin()
	if err != nil {
		return err
	}

	defer rollback(tx)

	stmt, err := psdb.DB.Prepare("INSERT or REPLACE INTO ttl (id, created, expires) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = tx.Stmt(stmt).Exec(id, time.Now().Unix(), expiration)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetTTLByID -- Find the TTL in the database by id and return it
func (psdb *PSDB) GetTTLByID(id string) (expiration int64, err error) {
	psdb.mtx.Lock()
	defer psdb.mtx.Unlock()

	err = psdb.DB.QueryRow(fmt.Sprintf(`SELECT expires FROM ttl WHERE id="%s"`, id)).Scan(&expiration)
	if err != nil {
		return 0, err
	}

	return expiration, nil
}

// DeleteTTLByID -- Find the TTL in the database by id and delete it
func (psdb *PSDB) DeleteTTLByID(id string) error {
	psdb.mtx.Lock()
	defer psdb.mtx.Unlock()

	tx, err := psdb.DB.Begin()
	if err != nil {
		return err
	}

	defer rollback(tx)

	stmt, err := tx.Prepare("DELETE FROM ttl WHERE id=?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = tx.Stmt(stmt).Exec(id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func rollback(tx *sql.Tx) {
	err := tx.Rollback()
	if err != nil && err != sql.ErrTxDone {
		log.Printf("%s\n", err)
	}
}
