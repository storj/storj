// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"storj.io/storj/pkg/piecestore"
)

// DBCleanup -- go routine to check ttl database for expired entries
// pass in database path and location of file for deletion
func DBCleanup(DBPath string, dir string) error {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	tickChan := time.NewTicker(time.Second * 5).C
	for {
		select {
		case <-tickChan:
			rows, err := db.Query(fmt.Sprintf("SELECT hash, expires FROM ttl WHERE expires < %d", time.Now().Unix()))
			if err != nil {
				return err
			}
			defer rows.Close()

			// iterate though selected rows
			// tried to wrap this inside (if rows != nil) but seems rows has value even if no entries meet condition. Thoughts?
			for rows.Next() {
				var expHash string
				var expires int64

				err = rows.Scan(&expHash, &expires)
				if err != nil {
					return err
				}

				// delete file on local machine
				err = pstore.Delete(expHash, dir)
				if err != nil {
					return err
				}
				log.Printf("Deleted file: %s\n", expHash)
			}

			// getting error when attempting to delete DB entry while inside it, so deleting outside for loop. Thoughts?
			_, err = db.Exec(fmt.Sprintf("DELETE FROM ttl WHERE expires < %d", time.Now().Unix()))
			if err != nil {
				return err
			}
		}
	}
}

// AddTTLToDB -- Insert TTL into database by hash
func AddTTLToDB(DBPath string, hash string, ttl int64) error {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf(`INSERT INTO ttl (hash, created, expires) VALUES ("%s", "%d", "%d")`, hash, time.Now().Unix(), ttl))
	if err != nil {
		return err
	}

	return nil
}

// CreateDB -- Create TTL database and table
func CreateDB(DBPath string) error {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`hash` TEXT UNIQUE, `created` INT(10), `expires` INT(10));")
	if err != nil {
		return err
	}

	return nil
}

// GetTTLByHash -- Find the TTL in the database by hash and return it
func GetTTLByHash(DBPath string, hash string) (ttl int64, err error) {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

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
