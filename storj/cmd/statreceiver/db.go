// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var (
	sqlupsert = map[string]string{
		"sqlite3": "INSERT INTO metrics (metric, instance, val, timestamp) " +
			"VALUES (?, ?, ?, ?) ON CONFLICT(metric, instance) DO UPDATE SET " +
			"val=excluded.val, timestamp=excluded.timestamp;",
		"postgres": "INSERT INTO metrics (metric, instance, val, timestamp) " +
			"VALUES ($1, $2, $3, $4) ON CONFLICT(metric, instance) DO UPDATE SET " +
			"val=EXCLUDED.val, timestamp=EXCLUDED.timestamp;",
	}
)

// DBDest is a database metric destination. It stores the latest value given
// a metric key and application per instance.
type DBDest struct {
	driver  string
	address string

	mu sync.Mutex
	db *sql.DB
}

// NewDBDest creates a DBDest
func NewDBDest(driver, address string) *DBDest {
	if _, found := sqlupsert[driver]; !found {
		panic(fmt.Sprintf("driver %s not supported", driver))
	}
	return &DBDest{
		driver:  driver,
		address: address,
	}
}

// Metric implements the MetricDest interface
func (db *DBDest) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	db.mu.Lock()
	if db.db == nil {
		conn, err := sql.Open(db.driver, db.address)
		if err != nil {
			db.mu.Unlock()
			return err
		}
		db.db = conn
	}
	db.mu.Unlock()

	_, err := db.db.Exec(sqlupsert[db.driver], application+"."+string(key), instance, val, ts.Unix())
	return err
}
