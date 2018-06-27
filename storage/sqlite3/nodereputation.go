// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"database/sql"
	"fmt"
	"strings"

	rep "storj.io/storj/pkg/nodereputation"
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

// StartDBError is an error class for errors related to the reputation package
var StartDBError = errs.Class("reputation start sqlite3 error")

// startDB starts a sqlite3 database from the file path parameter
func startDB(filePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, StartDBError.Wrap(err)
	}

	return db, nil
}

// createReputationTable creates a table in sqlite3 based on the create table string parameter
func createReputationTable(db *sql.DB) error {

	pre := func(s string) string {
		return fmt.Sprintf(`%s_good_recall REAL,
			%s_bad_recall REAL,
			%s_feature_counter REAL,
			%s_weight_denominator REAL,
			%s_cumulative_sum_reputation REAL,
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

type paramValue struct {
	param proto.Parameter
	val   float64
}

// type stateValue struct {
// 	state proto.BetaStateCols
// 	val   proto.UpdateRepValue
// }

func createNewNodeRecord(db *sql.DB, nodeName string, params []paramValue) error {
	insertNewNodeName(db, nodeName)
	for _, feature := range proto.Feature_name {
		for _, pair := range params {
			// err :=
			updateNodeParameters(db, nodeName, feature, pair.param, pair.val)
			// if err != nil {
			// 	panic(err)
			// }
		}
	}

	return nil
}

func insertNewNodeName(db *sql.DB, nodeName string) error {
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

type nodeFeature struct {
	nodeName          string
	feature           string
	goodRecall        float64
	badRecall         float64
	featureCounter    float64
	weightDenominator float64
	cumulativeSum     float64
	reputation        float64
}

func selectNodeFeature(db *sql.DB, nodeName string, col proto.Feature) (nodeFeature, error) {
	var res nodeFeature

	rows, err := db.Query(selectFeatureStmt(col, nodeName))
	if err != nil {
		return res, SelectError.Wrap(err)
	}
	defer rows.Close()

	res, err = selectedFeaturesToNodeRecord(rows)
	if err != nil {
		return res, SelectError.Wrap(err)
	}

	res.nodeName = nodeName
	res.feature = col.String()

	err = rows.Err()
	if err != nil {
		return res, SelectError.Wrap(err)
	}

	return res, nil
}

//
func getRep(db *sql.DB, nodeName string) ([]nodeFeature, error) {
	var res []nodeFeature
	updateRes := func(res []nodeFeature, feature proto.Feature) ([]nodeFeature, error) {
		newRes, err := selectNodeFeature(db, nodeName, proto.Feature_UPTIME)
		if err != nil {
			return nil, SelectError.Wrap(err)
		}
		res = append(res, newRes)
		return res, nil
	}

	for i := range proto.Feature_name {
		switch i {
		case 0:
			updateRes(res, proto.Feature_UPTIME)
		case 1:
			updateRes(res, proto.Feature_AUDIT)
		case 2:
			updateRes(res, proto.Feature_LATENCY)
		case 3:
			updateRes(res, proto.Feature_AMOUNT_OF_DATA_STORED)
		case 4:
			updateRes(res, proto.Feature_FALSE_CLAIMS)
		case 5:
			updateRes(res, proto.Feature_SHARDS_MODIFIED)
		}
	}
	return res, nil
}

func matchRepOrderStmt(features []proto.Feature, notIn []string) string {
	var exclude []string

	for _, not := range notIn {
		exclude = append(exclude, fmt.Sprintf(`'%s'`, not))
	}

	var ordered []string

	for _, feature := range proto.Feature_name {
		ordered = append(ordered, fmt.Sprintf("%s DESC", feature))
	}

	selectNodesStmt := fmt.Sprintf(`SELECT node_name
	FROM node_reputation
	WHERE node_name NOT IN (%s)
	ORDER BY %s`, strings.Join(exclude, ","), strings.Join(ordered, ","))

	return selectNodesStmt
}

//
func matchRepOrder(db *sql.DB, features []proto.Feature, notIn []string) ([]string, error) {
	rows, err := db.Query(matchRepOrderStmt(features, notIn))
	if err != nil {
		return nil, SelectError.Wrap(err)
	}
	defer rows.Close()

	var res []string

	for rows.Next() {
		var s string
		err := rows.Scan(s)
		if err != nil {
			return nil, SelectError.Wrap(err)
		}

		res = append(res, s)
	}

	return res, nil

}

func selectAllBetaStateStmt() string {
	res := "SELECT"
	fromWhere := `FROM node_reputation
	WHERE node_name = ?`

	pre := func(f string) string {
		return fmt.Sprintf(`
			%s_feature_counter,
			%s_cumulative_sum_reputation,
			%s_current_reputation`, f, f, f)
	}

	var repState []string

	for _, v := range proto.Feature_name {
		repState = append(repState, pre(v))
	}

	joined := strings.Join(repState, ",")

	res = res + joined + fromWhere

	return res
}

//
func updateNodeRecord(db *sql.DB, nodeName string, col proto.Feature, value proto.UpdateRepValue) error {
	node, err := selectNodeFeature(db, nodeName, col)
	if err != nil {
		return UpdateError.Wrap(err)
	}
	betaRes := rep.Beta(node.badRecall, node.goodRecall, node.weightDenominator, node.featureCounter, node.cumulativeSum, updateToFloat(value))
	newSum := node.cumulativeSum + betaRes.Reputation
	newCount := node.featureCounter + 1

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

	_, err = updateStmt.Exec(newCount, newSum, betaRes.Reputation, nodeName)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	return tx.Commit()
}

func updateNodeParameters(db *sql.DB, nodeName string, feature string, parameter proto.Parameter, parameterValue float64) error {
	tx, err := db.Begin()
	if err != nil {
		return UpdateError.Wrap(err)
	}
	defer tx.Rollback()

	updateParamString := fmt.Sprintf(`UPDATE node_reputation
	 SET %s_%s = %.4f
	 WHERE node_name = '%s';`, feature, parameter.String(), parameterValue, nodeName)

	_, err = tx.Exec(updateParamString)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	return tx.Commit()
}

// assumtion one row per node id
func selectedFeaturesToNodeRecord(rows *sql.Rows) (nodeFeature, error) {
	var res nodeFeature

	for rows.Next() {
		err := rows.Scan(
			&res.goodRecall,
			&res.badRecall,
			&res.featureCounter,
			&res.weightDenominator,
			&res.cumulativeSum,
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
		return fmt.Sprintf(`%s_feature_counter = ?,
			%s_cumulative_sum_reputation = ?,
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
			%s_feature_counter,
			%s_weight_denominator,
			%s_cumulative_sum_reputation,
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

func updateToFloat(val proto.UpdateRepValue) float64 {
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
