// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package cockroachkv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	_ "github.com/lib/pq"
	"storj.io/storj/internal/dbutil/crdbutil/crdbtest"
	"storj.io/storj/storage/testsuite"
)

var ctx = context.Background() // test context

func newTestCockroachDB(t testing.TB) (store *Client, cleanup func()) {
	if *crdbtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", crdbtest.DefaultConnStr)
	}

	pgdb, err := New(*crdbtest.ConnStr)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	return pgdb, func() {
		if err := pgdb.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	}
}

func TestSuite(t *testing.T) {
	store, cleanup := newTestCockroachDB(t)
	defer cleanup()

	testsuite.RunTests(t, store)
}

func TestUTF8(t *testing.T) {
	pgConn, err := sql.Open("postgres", "postgres://root@localhost:26257/teststorj?sslmode=disable")
	if err != nil {
		t.Error(err)
	}

	bucket := []byte{}
	key := []byte("full/path/2")
	// oldValue := []byte{0, 255, 255, 1}
	// newValue := []byte{0, 255, 255, 4}
	oldValue := []byte("\x00\xFF\xFF\x01")
	newValue := []byte("\x00\xFF\xFF\x04")

	insertResult, err := pgConn.Exec("INSERT INTO pathdata (bucket, fullpath, metadata) VALUES ($1::BYTEA, $2::BYTEA, $3::BYTEA);", bucket, key, oldValue)
	if err != nil {
		t.Error(err)
	}
	rowsAffected, err := insertResult.RowsAffected()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("ROWS AFFECTED: %d\n", rowsAffected)

	q := `
	WITH matching_key AS (
		SELECT * FROM pathdata WHERE bucket = $1::BYTEA AND fullpath = $2::BYTEA
	), updated AS (
		UPDATE pathdata
			SET metadata = $4:::BYTEA
			FROM matching_key mk
			WHERE pathdata.metadata = $3:::BYTEA
				AND pathdata.bucket = mk.bucket
				AND pathdata.fullpath = mk.fullpath
			RETURNING 1
	)
	SELECT EXISTS(SELECT 1 FROM matching_key) AS key_present, EXISTS(SELECT 1 FROM updated) AS value_updated;
	`

	row := pgConn.QueryRow(q, bucket, key, oldValue, newValue)

	var keyPresent, valueUpdated bool
	err = row.Scan(&keyPresent, &valueUpdated)
	if err != nil {
		t.Error(err)
	}
	if !keyPresent {
		t.Error(errors.New("key not found"))
	}
	if !valueUpdated {
		t.Error(errors.New("value changed"))
	}
}
