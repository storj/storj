// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ttl

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3" // sqlite is weird and needs underscore

	"storj.io/storj/pkg/piecestore"
)

// TTL -- ttl database
type TTL struct {
	DB *sql.DB
}

// NewTTL -- creates ttl database and struct
func NewTTL(DBPath string) (*TTL, error) {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS `ttl` (`id` TEXT UNIQUE, `created` INT(10), `expires` INT(10));")
	if err != nil {
		return nil, err
	}

	return &TTL{db}, nil
}

// checkEntries -- checks for and deletes expired TTL entries
func checkEntries(dir string, rows *sql.Rows) error {

	for rows.Next() {
		var expID string
		var expires int64

		err := rows.Scan(&expID, &expires)
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
func (ttl *TTL) DBCleanup(dir string) error {

	tickChan := time.NewTicker(time.Second * 5).C
	for {
		select {
		case <-tickChan:
			now := time.Now().Unix()

			rows, err := ttl.DB.Query(fmt.Sprintf("SELECT id, expires FROM ttl WHERE expires < %d", now))
			if err != nil {
				return err
			}
			defer rows.Close()

			if err := checkEntries(dir, rows); err != nil {
				return err
			}

			_, err = ttl.DB.Exec(fmt.Sprintf("DELETE FROM ttl WHERE expires < %d", now))
			if err != nil {
				return err
			}
		}
	}
}

// AddTTLToDB -- Insert TTL into database by id
func (ttl *TTL) AddTTLToDB(id string, expiration int64) error {

	_, err := ttl.DB.Exec(fmt.Sprintf(`INSERT INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, id, time.Now().Unix(), expiration))
	if err != nil {
		return err
	}

	return nil
}

// GetTTLByID -- Find the TTL in the database by id and return it
func (ttl *TTL) GetTTLByID(id string) (expiration int64, err error) {

	rows, err := ttl.DB.Query(fmt.Sprintf(`SELECT expires FROM ttl WHERE id="%s"`, id))
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
func (ttl *TTL) DeleteTTLByID(id string) error {

	_, err := ttl.DB.Exec(fmt.Sprintf(`DELETE FROM ttl WHERE id="%s"`, id))
	return err
}
