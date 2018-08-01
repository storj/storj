// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	_ "github.com/mattn/go-sqlite3" // sqlite is weird and needs underscore

	"storj.io/storj/pkg/piecestore"
	pb "storj.io/storj/protos/piecestore"
)

// PSDB -- Piecestore database
type PSDB struct {
	DB  *sql.DB
	mtx sync.Mutex
}

// OpenPSDB -- opens PSDB at DBPath
func OpenPSDB(DBPath string) (*PSDB, error) {
	if err := os.MkdirAll(filepath.Dir(DBPath), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=memory&mutex=full", DBPath))
	if err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

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

	return &PSDB{DB: db}, nil
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
		if rows.Err() != nil {
			return rows.Err()
		}
	}

	return nil
}

func (psdb *PSDB) Close() error {
	return psdb.DB.Close()
}

// CheckEntries -- go routine to check ttl database for expired entries
// pass in database and location of file for deletion
func (psdb *PSDB) CheckEntries(dir string) error {

	tickChan := time.NewTicker(time.Second * 5).C
	for {
		select {
		case <-tickChan:
			now := time.Now().Unix()

			rows, err := psdb.DB.Query(fmt.Sprintf("SELECT id FROM ttl WHERE expires < %d AND expires > 0", now))
			if err != nil {
				return err
			}
			defer func() {
				if err := rows.Close(); err != nil {
					log.Printf("failed to close Rows: %s\n", err)
				}
			}()

			if err := deleteEntriesFromFS(dir, rows); err != nil {
				return err
			}

			_, err = psdb.DB.Exec(fmt.Sprintf("DELETE FROM ttl WHERE expires < %d AND expires > 0", now))
			if err != nil {
				return err
			}
		}
	}
}

// WriteBandwidthAllocToDB -- Insert bandwidth agreement into DB
func (psdb *PSDB) WriteBandwidthAllocToDB(ba *pb.BandwidthAllocation) error {
	psdb.mtx.Lock()
	defer psdb.mtx.Unlock()

	data := ba.GetData()
	if data == nil {
		return nil
	}

	serialized, err := proto.Marshal(data)
	if err != nil {
		return err
	}

	tx, err := psdb.DB.Begin()
	if err != nil {
		return err
	}

	stmt, err := psdb.DB.Prepare(`INSERT INTO bandwidth_agreements (agreement, signature) VALUES (?, ?)`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = tx.Stmt(stmt).Exec(serialized, ba.GetSignature())
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Printf("%s\n", rollbackErr)
		}
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

	stmt, err := psdb.DB.Prepare("INSERT or REPLACE INTO ttl (id, created, expires) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = tx.Stmt(stmt).Exec(id, time.Now().Unix(), expiration)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Printf("%s\n", rollbackErr)
		}
		return err
	}

	return tx.Commit()
}

// GetTTLByID -- Find the TTL in the database by id and return it
func (psdb *PSDB) GetTTLByID(id string) (expiration int64, err error) {
	psdb.mtx.Lock()
	defer psdb.mtx.Unlock()

	rows, err := psdb.DB.Query(fmt.Sprintf(`SELECT expires FROM ttl WHERE id="%s"`, id))
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close Rows: %s\n", err)
		}
	}()

	for rows.Next() {
		err = rows.Scan(&expiration)
		if err != nil {
			return 0, err
		}
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

	stmt, err := tx.Prepare("DELETE FROM ttl WHERE id=?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = tx.Stmt(stmt).Exec(id)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Printf("%s\n", rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
