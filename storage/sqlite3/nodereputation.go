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

	pre := func(s string) string {
		return fmt.Sprintf(`SELECT
			%s_good_recall REAL,
			%s_bad_recall REAL,
			%s_weight_counter REAL,
			%s_weight_denominator REAL,
			%s_cumulative_mean_reputation REAL,
			%s_current_reputation REAL`, s, s, s, s, s, s)
	}

	timefmt := "%Y-%m-%d %H:%M:%f"

	createTableStmt := fmt.Sprintf(`CREATE table node_reputation (
		node_name TEXT NOT NULL,
		last_seen timestamp DEFAULT(STRFTIME('%s', 'NOW')) NOT NULL,
		%s, %s, %s, %s, %s, %s,
		PRIMARY KEY(node_name, last_seen));`,
		timefmt,
		pre("uptime"),
		pre("audit"),
		pre("latency"),
		pre("amount_of_data_stored"),
		pre("false_claims"),
		pre("shards_modified"),
	)

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
	SET last_seen = STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW'),`

	pre := func(f string) string {
		return fmt.Sprintf(`%s_weight_counter = ?,
			%s_cumulative_mean_reputation = ?,
			%s_current_reputation = ?
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
