// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"storj.io/storj/pkg/piecestore"
)

// CheckEntries -- checks for and deletes expired TTL entries
func CheckEntries(db *sql.DB, dir string, rows *sql.Rows) error {

	for rows.Next() {
		var expHash string
		var expires int64

		err := rows.Scan(&expHash, &expires)
		if err != nil {
			return err
		}

		// delete file on local machine
		err = pstore.Delete(expHash, dir)
		if err != nil {
			return err
		}

		log.Printf("Deleted file: %s\n", expHash)
		if rows.Err() != nil {

			return rows.Err()
		}
	}

	return nil
}

// DBCleanup -- go routine to check ttl database for expired entries
// pass in database and location of file for deletion
func DBCleanup(db *sql.DB, dir string) error {

	tickChan := time.NewTicker(time.Second * 5).C
	for {
		select {
		case <-tickChan:
			rows, err := db.Query(fmt.Sprintf("SELECT hash, expires FROM ttl WHERE expires < %d", time.Now().Unix()))
			if err != nil {
				return err
			}

			err = CheckEntries(db, dir, rows)
			if err != nil {
				rows.Close()
				return err
			}
			rows.Close()

			_, err = db.Exec(fmt.Sprintf("DELETE FROM ttl WHERE expires < %d", time.Now().Unix()))
			if err != nil {
				return err
			}
		}
	}
}

// AddTTLToDB -- Insert TTL into database by hash
func AddTTLToDB(db *sql.DB, hash string, ttl int64) error {

	_, err := db.Exec(fmt.Sprintf(`INSERT INTO ttl (hash, created, expires) VALUES ("%s", "%d", "%d")`, hash, time.Now().Unix(), ttl))
	if err != nil {
		return err
	}

	return nil
}

// CreateDB -- Create TTL database and table
func CreateDB(db *sql.DB) error {

	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`hash` TEXT UNIQUE, `created` INT(10), `expires` INT(10));")
	if err != nil {
		return err
	}

	return nil
}

// GetTTLByHash -- Find the TTL in the database by hash and return it
func GetTTLByHash(db *sql.DB, hash string) (ttl int64, err error) {

	rows, err := db.Query(fmt.Sprintf(`SELECT expires FROM ttl WHERE hash="%s"`, hash))
	if err != nil {
		return 0, err
	}

	for rows.Next() {
		err = rows.Scan(&ttl)
		if err != nil {
			return 0, err
		}
	}

	return ttl, nil
}

// DeleteTTLByHash -- Find the TTL in the database by hash and delete it
func DeleteTTLByHash(db *sql.DB, hash string) error {

	_, err := db.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE hash="%s"`, hash))
	return err
}
