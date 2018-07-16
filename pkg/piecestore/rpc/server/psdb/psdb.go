// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // sqlite is weird and needs underscore

	"storj.io/storj/pkg/piecestore"
	pb "storj.io/storj/protos/piecestore"
)

// PSDB -- Piecestore database
type PSDB struct {
	DB *sql.DB
}

// NewPSDB -- creates
func NewPSDB(DBPath string) (*PSDB, error) {
	if err := os.MkdirAll(filepath.Dir(DBPath), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`id` TEXT UNIQUE, `created` INT(10), `expires` INT(10));")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS `bandwidth_agreements` (`payer` TEXT UNIQUE, `client` TEXT UNIQUE, `size` INT);")
	if err != nil {
		return nil, err
	}

	return &PSDB{db}, nil
}

// checkEntries -- checks for and deletes expired TTL entries
func checkEntries(dir string, rows *sql.Rows) error {

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

// DBCleanup -- go routine to check ttl database for expired entries
// pass in database and location of file for deletion
func (psdb *PSDB) DBCleanup(dir string) error {

	tickChan := time.NewTicker(time.Second * 5).C
	for {
		select {
		case <-tickChan:
			now := time.Now().Unix()

			rows, err := psdb.DB.Query(fmt.Sprintf("SELECT id FROM ttl WHERE expires < %d AND expires > 0", now))
			if err != nil {
				return err
			}
			defer rows.Close()

			if err := checkEntries(dir, rows); err != nil {
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

	_, err := psdb.DB.Exec(fmt.Sprintf(`INSERT INTO bandwidth_agreements (payer, client, size) VALUES ("%s", "%s", "%d")`, ba.Data.Payer, ba.Data.Client, ba.Data.Size))
	if err != nil {
		return err
	}

	log.Printf("Payer: %s, Client: %s, Size: %v\n", ba.Data.Payer, ba.Data.Client, ba.Data.Size)

	return nil
}

// AddTTLToDB -- Insert TTL into database by id
func (psdb *PSDB) AddTTLToDB(id string, expiration int64) error {

	_, err := psdb.DB.Exec(fmt.Sprintf(`INSERT or REPLACE INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, id, time.Now().Unix(), expiration))
	return err
}

// GetTTLByID -- Find the TTL in the database by id and return it
func (psdb *PSDB) GetTTLByID(id string) (expiration int64, err error) {

	rows, err := psdb.DB.Query(fmt.Sprintf(`SELECT expires FROM ttl WHERE id="%s"`, id))
	if err != nil {
		return 0, err
	}

	defer rows.Close()

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

	_, err := psdb.DB.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, id))
	return err
}
