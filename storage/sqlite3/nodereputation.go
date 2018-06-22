// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"database/sql"
	"fmt"

	proto "storj.io/storj/protos/nodereputation"

	// import of sqlite3 for side effects
	_ "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"
)

// CreateTableError is an error class for errors related to the reputation package
var CreateTableError = errs.Class("reputation table creation error")

// CreateNodeError is an error class for errors related to the reputation package
var CreateNodeError = errs.Class("reputation node creation error")

// SelectError is an error class for errors related to the reputation package
var SelectError = errs.Class("reputation selection error")

// UpdateError is an error class for errors related to the reputation package
var UpdateError = errs.Class("reputation update error")

// createTable creates a table in sqlite3 based on the create table string parameter
func createTable(db *sql.DB) error {

	createTableStmt := `CREATE table node_reputation (
		node_name TEXT NOT NULL,
		last_seen timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')) NOT NULL,

		uptime_good_recall REAL,
		uptime_bad_recall REAL,
		uptime_weight_counter REAL,
		uptime_weight_denominator REAL,
		uptime_cumulative_mean_reputation REAL,
	    uptime_current_reputation REAL,

		audit_good_recall REAL,
		audit_bad_recall REAL,
		audit_weight_counter REAL,
		audit_weight_denominator REAL,
		audit_cumulative_mean_reputation REAL,
	    audit_current_reputation REAL,

		latency_good_recall REAL,
		latency_bad_recall REAL,
		latency_weight_counter REAL,
		latency_weight_denominator REAL,
		latency_cumulative_mean_reputation REAL,
	    latency_current_reputation REAL,

		amount_of_data_stored_good_recall REAL,
		amount_of_data_stored_bad_recall REAL,
		amount_of_data_stored_weight_counter REAL,
		amount_of_data_stored_weight_denominator REAL,
		amount_of_data_stored_cumulative_mean_reputation REAL,
	    amount_of_data_stored_current_reputation REAL,

		false_claims_good_recall REAL,
		false_claims_bad_recall REAL,
		false_claims_weight_counter REAL,
		false_claims_weight_denominator REAL,
		false_claims_cumulative_mean_reputation REAL,
	    false_claims_current_reputation REAL,

		shards_modified_good_recall REAL,
		shards_modified_bad_recall REAL,
		shards_modified_weight_counter REAL,
		shards_modified_weight_denominator REAL,
		shards_modified_cumulative_mean_reputation REAL,
	    shards_modified_current_reputation REAL,

	PRIMARY KEY(node_name, last_seen)
	);`

	_, err := db.Exec(createTableStmt)
	if err != nil {
		return CreateTableError.Wrap(err)
	}
	return nil
}

func createNewNodeRecord(db *sql.DB, nodeName string) error {
	tx, err := db.Begin()
	if err != nil {
		return CreateNodeError.Wrap(err)
	}
	defer tx.Rollback()

	createNodeString := `INSERT
	INTO node_reputation (node_name) values (?);`

	insertStmt, err := tx.Prepare(createNodeString)
	if err != nil {
		return CreateNodeError.Wrap(err)
	}
	defer insertStmt.Close()

	_, err = insertStmt.Exec(nodeName)
	if err != nil {
		return CreateNodeError.Wrap(err)
	}

	return tx.Commit()
}

func beta(x float64) float64 {
	return float64(42)
}

type nodeRecord struct {
	goodRecall        float64
	badRecall         float64
	weightCounter     float64
	weightDenominator float64
	meanReputation    float64
	reputation        float64
}

func selectNodeFeature(db *sql.DB, nodeName string, col proto.Feature) (nodeRecord, error) {
	var res nodeRecord

	rows, err := db.Query(selectFeatureStmt(col, nodeName))
	if err != nil {
		return res, SelectError.Wrap(err)
	}
	defer rows.Close()

	res, err = selectFeaturesToNodeRecord(rows)
	if err != nil {
		return res, SelectError.Wrap(err)
	}

	err = rows.Err()
	if err != nil {
		return res, SelectError.Wrap(err)
	}

	return res, nil
}

func updateNodeRecord(db *sql.DB, nodeName string, col proto.Feature, value proto.UpdateValue) error {
	node, err := selectNodeFeature(db, nodeName, col)
	if err != nil {
	}
	newRep := beta(updateToFloat(value))
	newMean := (node.meanReputation + newRep) / 2
	newCount := node.weightCounter + 1

	tx, err := db.Begin()
	if err != nil {
		return UpdateError.Wrap(err)
	}
	defer tx.Rollback()

	updateString := updateFeatureRepStmt(col)

	updateStmt, err := tx.Prepare(updateString)
	if err != nil {
		return UpdateError.Wrap(err)
	}
	defer updateStmt.Close()

	_, err = updateStmt.Exec(newCount, newMean, newRep, nodeName)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	return tx.Commit()
}

func updateNodeParameters(db *sql.DB, goodRecall float64, badRecall float64, weightDenominator float64) {

}

// assumtion one row per node id
func selectFeaturesToNodeRecord(rows *sql.Rows) (nodeRecord, error) {
	var res nodeRecord

	for rows.Next() {
		err := rows.Scan(
			&res.goodRecall,
			&res.badRecall,
			&res.weightCounter,
			&res.weightDenominator,
			&res.meanReputation,
			&res.reputation,
		)
		if err != nil {
			return res, SelectError.Wrap(err)
		}
	}

	return res, nil
}

func updateFeatureRepStmt(feature proto.Feature) string {
	res := `UPDATE node_reputation
	SET last_seen = STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')`

	pre := func(f string) string {
		return fmt.Sprintf(`, %s_weight_counter = ?
			, %s_cumulative_mean_reputation = ?
			, %s_current_reputation = ?
			WHERE node_name = '?';`, f, f, f)
	}

	switch feature {
	case 0:
		res = res + pre("uptime")
	case 1:
		res = res + pre("audit")
	case 2:
		res = res + pre("latency")
	case 3:
		res = res + pre("amount_of_data_stored")
	case 4:
		res = res + pre("false_claims")
	case 5:
		res = res + pre("shards_modified")
	}

	return res
}

func selectFeatureStmt(f proto.Feature, nodeName string) string {

	pre := func(s string) string {
		return fmt.Sprintf(`SELECT
			%s_good_recall,
			%s_bad_recall,
			%s_weight_counter,
			%s_weight_denominator,
			%s_cumulative_mean_reputation,
			%s_current_reputation`, s, s, s, s, s, s)
	}

	res := ""

	switch f {
	case 0:
		res = pre("uptime")
	case 1:
		res = pre("audit")
	case 2:
		res = pre("latency")
	case 3:
		res = pre("amount_of_data_stored")
	case 4:
		res = pre("false_claims")
	case 5:
		res = pre("shards_modified")
	}

	return res + "WHERE node_name = '" + nodeName + "';"
}

func updateToFloat(val proto.UpdateValue) float64 {
	res := float64(0)

	switch val {
	case 0:
		res = float64(-1)
	case 1:
		res = float64(-0.5)
	case 2:
		res = float64(-0.25)
	case 3:
		res = float64(0)
	case 4:
		res = float64(0.25)
	case 5:
		res = float64(0.5)
	case 6:
		res = float64(1)
	}

	return res
}
