// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"database/sql"

	// import of sqlite3 for side effects
	_ "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"
)

// CreateTableError is an error class for errors related to the reputation package
var CreateTableError = errs.Class("reputation table creation error")

// createTable creates a table in sqlite3 based on the create table string parameter
func createTable(db *sql.DB) error {

	var createStmt = `CREATE table node_reputation (
		node_name TEXT NOT NULL,
		last_seen timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')) NOT NULL,

		uptime_good_recall REAL,
		uptime_bad_recall REAL,
		uptime_weight_counter REAL,
		uptime_weight REAL,
		uptime_cumulative_mean_reputation REAL,
	    uptime_current_reputation REAL,

		audit_good_recall REAL,
		audit_bad_recall REAL,
		audit_weight_counter REAL,
		audit_weight REAL,
		audit_cumulative_mean_reputation REAL,
	    audit_current_reputation REAL,

		latency_good_recall REAL,
		latency_bad_recall REAL,
		latency_weight_counter REAL,
		latency_weight REAL,
		latency_cumulative_mean_reputation REAL,
	    latency_current_reputation REAL,

		amount_of_data_stored_good_recall REAL,
		amount_of_data_stored_bad_recall REAL,
		amount_of_data_stored_weight_counter REAL,
		amount_of_data_stored_weight REAL,
		amount_of_data_stored_cumulative_mean_reputation REAL,
	    amount_of_data_stored_current_reputation REAL,

		false_claims_good_recall REAL,
		false_claims_bad_recall REAL,
		false_claims_weight_counter REAL,
		false_claims_weight REAL,
		false_claims_cumulative_mean_reputation REAL,
	    false_claims_current_reputation REAL,

		shards_modified_good_recall REAL,
		shards_modified_bad_recall REAL,
		shards_modified_weight_counter REAL,
		shards_modified_weight REAL,
		shards_modified_cumulative_mean_reputation REAL,
	    shards_modified_current_reputation REAL,

	PRIMARY KEY(node_name, last_seen)
	);`

	_, err := db.Exec(createStmt)
	if err != nil {
		return CreateTableError.Wrap(err)
	}
	return nil
}
