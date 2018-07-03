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

// StartDB starts a sqlite3 database from the file path parameter
func StartDB(filePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, StartDBError.Wrap(err)
	}

	err = createReputationTable(db)
	if err != nil {
		return nil, StartDBError.Wrap(err)
	}

	return db, nil
}

// createReputationTable creates a table in sqlite3 based on the create table string parameter
func createReputationTable(db *sql.DB) error {
	var res []string

	for _, feature := range proto.Feature_name {
		for _, param := range proto.RepParameter_name {
			res = append(res, fmt.Sprintf("%s_%s", feature, param))
		}
		for _, state := range proto.RepState_name {
			res = append(res, fmt.Sprintf("%s_%s", feature, state))
		}
	}

	timefmt := "%Y-%m-%d %H:%M:%f"

	createTableStmt := fmt.Sprintf(`CREATE table node_reputation (
		node_name TEXT NOT NULL,
		last_seen timestamp DEFAULT(STRFTIME('%s', 'NOW')) NOT NULL,
		%s,
		PRIMARY KEY(node_name, last_seen));`,
		timefmt,
		strings.Join(res, ",\n"),
	)

	_, err := db.Exec(createTableStmt)
	if err != nil {
		return CreateTableError.Wrap(err)
	}
	return nil
}

// CreateNewNodeRecord creates a new record for a node in the database, not idempotent
func CreateNewNodeRecord(db *sql.DB, nodeName string) error {

	type paramValue struct {
		param proto.RepParameter
		val   float64
	}

	type stateValue struct {
		state proto.RepState
		val   proto.UpdateRepValue
	}

	// default values
	params := []paramValue{
		paramValue{
			param: proto.RepParameter_BAD_RECALL,
			val:   0.995,
		},
		paramValue{
			param: proto.RepParameter_GOOD_RECALL,
			val:   0.99,
		},
		paramValue{
			param: proto.RepParameter_WEIGHT_DENOMINATOR,
			val:   10000.0,
		},
	}
	states := []stateValue{
		stateValue{
			state: proto.RepState_CUMULATIVE_SUM_REPUTATION,
			val:   proto.UpdateRepValue_ZERO,
		},
		stateValue{
			state: proto.RepState_CURRENT_REPUTATION,
			val:   proto.UpdateRepValue_ZERO,
		},
		stateValue{
			state: proto.RepState_FEATURE_COUNTER,
			val:   proto.UpdateRepValue_ZERO,
		},
	}

	insertNewNodeName(db, nodeName)
	for _, feature := range proto.Feature_name {
		for _, pair := range params {
			err := updateNodeParameters(db, nodeName, feature, pair.param, pair.val)
			if err != nil {
				CreateNodeError.Wrap(err)
			}
		}

		for _, state := range states {
			err := updateNodeState(db, nodeName, feature, state.state, state.val)
			if err != nil {
				CreateNodeError.Wrap(err)
			}
		}
	}

	return nil
}

// insertNewNodeName inserts a row into the database with the provied name, not idempotent
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

// NodeFeature is a GO type to represent a single feature from the database
type NodeFeature struct {
	nodeName           string
	feature            string
	goodRecall         float64
	badRecall          float64
	featureGoodCounter float64
	featureCounter     float64
	weightDenominator  float64
	cumulativeSum      float64
	reputation         float64
}

// selectNodeFeature is a function used to select a single feature for a given node name
func selectNodeFeature(db *sql.DB, nodeName string, feature string) (NodeFeature, error) {
	var res NodeFeature

	stmt := selectFeatureStmt(feature, nodeName)
	rows, err := db.Query(stmt)
	if err != nil {
		return res, SelectError.Wrap(err)
	}
	defer rows.Close()

	res, err = selectedFeaturesToNodeRecord(rows)
	if err != nil {
		return res, SelectError.Wrap(err)
	}

	res.nodeName = nodeName
	res.feature = feature

	err = rows.Err()
	if err != nil {
		return res, SelectError.Wrap(err)
	}

	return res, nil
}

// GetRep is a function to retrive the reputation for a given node name from the database
func GetRep(db *sql.DB, nodeName string) ([]NodeFeature, error) {
	var res []NodeFeature
	updateRes := func(feature string) (NodeFeature, error) {
		var newRes NodeFeature
		newRes, err := selectNodeFeature(db, nodeName, feature)
		if err != nil {
			return newRes, SelectError.Wrap(err)
		}
		return newRes, nil
	}

	for _, feature := range proto.Feature_name {
		update, err := updateRes(feature)
		if err != nil {
			SelectError.Wrap(err)
		}
		res = append(res, update)
	}
	return res, nil
}

// matchRepOrderStmt is a function that generates a string for filtering nodes from the database
func matchRepOrderStmt(limit int64, features []proto.Feature, state proto.RepState, notIn []string) string {
	var exclude []string

	for _, not := range notIn {
		exclude = append(exclude, fmt.Sprintf(`'%s'`, not))
	}

	var ordered []string

	for _, feature := range features {
		ordered = append(ordered,
			fmt.Sprintf("%s_%s DESC",
				feature.String(), state.String()))
	}

	selectNodesStmt := fmt.Sprintf(`SELECT node_name
	FROM node_reputation
	WHERE node_name NOT IN (%s)
	ORDER BY %s
	LIMIT %d;`, strings.Join(exclude, ","), strings.Join(ordered, ",\n"), limit)

	return selectNodesStmt
}

// matchRepOrderStmt is a function that looks for nodes that satisfy the constraint parameters
func matchRepOrder(db *sql.DB, limit int64, features []proto.Feature, notIn []string) ([]string, error) {
	stmt := matchRepOrderStmt(limit, features, proto.RepState_CURRENT_REPUTATION, notIn)
	rows, err := db.Query(stmt)
	if err != nil {
		return nil, SelectError.Wrap(err)
	}
	defer rows.Close()

	var res []string

	for rows.Next() {
		var s string
		err := rows.Scan(&s)
		if err != nil {
			return nil, SelectError.Wrap(err)
		}

		res = append(res, s)
	}

	return res, nil
}

// updateNodeRecord is a function that updates a single node's feature
func updateNodeRecord(db *sql.DB, nodeName string, feature proto.Feature, value proto.UpdateRepValue) error {
	node, err := selectNodeFeature(db, nodeName, feature.String())
	if err != nil {
		return UpdateError.Wrap(err)
	}

	newValue := updateToFloat(value)
	betaRes := rep.Beta(node.badRecall, node.goodRecall, node.weightDenominator, node.featureCounter, node.cumulativeSum, newValue)
	newRep := betaRes.Reputation
	newSum := node.cumulativeSum + newRep
	newCount := node.featureCounter + 1.0

	newGoodCount := 0.0
	if value > 0 {
		newGoodCount = newGoodCount + node.featureGoodCounter + 1.0
	} else {
		newGoodCount = newGoodCount + node.featureGoodCounter + 0.0
	}

	tx, err := db.Begin()
	if err != nil {
		return UpdateError.Wrap(err)
	}
	defer tx.Rollback()

	updateStringRep := updateFeatureRepStmt(nodeName, feature.String(), proto.RepState_CURRENT_REPUTATION.String(), newRep)
	_, err = tx.Exec(updateStringRep)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	updateStringSum := updateFeatureRepStmt(nodeName, feature.String(), proto.RepState_CUMULATIVE_SUM_REPUTATION.String(), newSum)
	_, err = tx.Exec(updateStringSum)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	updateStringCount := updateFeatureRepStmt(nodeName, feature.String(), proto.RepState_FEATURE_COUNTER.String(), newCount)
	_, err = tx.Exec(updateStringCount)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	updateStringGoodCount := updateFeatureRepStmt(nodeName, feature.String(), proto.RepState_FEATURE_GOOD_COUNTER.String(), newGoodCount)
	_, err = tx.Exec(updateStringGoodCount)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	return tx.Commit()
}

// updateNodeParameters is a function that updates a node's state values
func updateNodeParameters(db *sql.DB, nodeName string, feature string, parameter proto.RepParameter, parameterValue float64) error {
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

// updateNodeState is a function that updates a node's state values
func updateNodeState(db *sql.DB, nodeName string, feature string, state proto.RepState, stateValue proto.UpdateRepValue) error {
	tx, err := db.Begin()
	if err != nil {
		return UpdateError.Wrap(err)
	}
	defer tx.Rollback()

	updateParamString := fmt.Sprintf(`UPDATE node_reputation
	 SET %s_%s = %.4f
	 WHERE node_name = '%s';`, feature, state.String(), updateToFloat(stateValue), nodeName)

	_, err = tx.Exec(updateParamString)
	if err != nil {
		return UpdateError.Wrap(err)
	}

	return tx.Commit()
}

// selectedFeaturesToNodeRecord converts a row to a NodFeature struct, assumtion one row per node id
func selectedFeaturesToNodeRecord(rows *sql.Rows) (NodeFeature, error) {
	var res NodeFeature

	for rows.Next() {
		err := rows.Scan(
			&res.goodRecall,
			&res.badRecall,
			&res.weightDenominator,
			&res.featureGoodCounter,
			&res.featureCounter,
			&res.cumulativeSum,
			&res.reputation,
		)
		if err != nil {
			return res, SelectError.Wrap(err)
		}
	}

	return res, nil
}

// updateFeatureRepStmt a function that generates a string for the database to update a node's feature
func updateFeatureRepStmt(nodeName string, feature string, state string, value float64) string {
	update := `UPDATE node_reputation
	SET last_seen = STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW'),`

	return fmt.Sprintf(`%s
		%s_%s = %.4f
		WHERE node_name = '%s';`, update, feature, state, value, nodeName)
}

// selectFeatureStmt a function that generates a string for the database to select a single feature of a node
func selectFeatureStmt(feature string, nodeName string) string {
	var cols []string

	for i := 0; i < len(proto.RepParameter_name); i++ {
		cols = append(cols, fmt.Sprintf("%s_%s", feature, proto.RepParameter_name[int32(i)]))
	}

	for i := 0; i < len(proto.RepState_name); i++ {
		cols = append(cols, fmt.Sprintf("%s_%s", feature, proto.RepState_name[int32(i)]))
	}

	return fmt.Sprintf(`SELECT
		%s FROM node_reputation WHERE node_name = '%s';`,
		strings.Join(cols, ",\n"), nodeName)
}

// updateToFloat converts a proto enum to a float
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
