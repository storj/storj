// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"database/sql"
	"os"

	"github.com/zeebo/errs"
	// import of sqlite3 for side effects
	_ "github.com/mattn/go-sqlite3"
)

// StartDBError is an error class for errors related to the reputation package
var StartDBError = errs.Class("reputation start sqlite3 error")

// CreateTableError is an error class for errors related to the reputation package
var CreateTableError = errs.Class("reputation table creation error")

// InsertError is an error class for errors related to the reputation package
var InsertError = errs.Class("reputation insertion error")

// SelectError is an error class for errors related to the reputation package
var SelectError = errs.Class("reputation selection error")

// IterError is an error class for errors related to the reputation package
var IterError = errs.Class("reputation iteration error")

// DeleteError is an error class for errors related to the reputation package
var DeleteError = errs.Class("reputation deletion error")

// nodeReputationRecord is the Data type for Rows in Reputation table
type nodeReputationRecord struct {
	source             string
	nodeName           string
	timestamp          string
	uptime             int64
	auditSuccess       int64
	auditFail          int64
	latency            int64
	amountOfDataStored int64
	falseClaims        int64
	shardsModified     int64
}

// column base type for nodeReputationRecord struct
type column string

// coproduct/sum type for the column type
const (
	sourceColumn             column = "source"
	nodeNameColumn           column = "nodeName"
	timestampColumn          column = "timestamp"
	uptimeColumn             column = "uptime"
	auditSuccessColumn       column = "auditSuccess"
	auditFailColumn          column = "auditFail"
	latencyColumn            column = "latency"
	amountOfDataStoredColumn column = "amountOfDataStored"
	falseClaimsColumn        column = "falseClaims"
	shardsModifiedColumn     column = "shardsModified"
)

// startDB starts a sqlite3 database from the file path parameter
func startDB(filePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, StartDBError.Wrap(err)
	}

	return db, nil
}

// EndServerDB cleans up the passed in database
func EndServerDB(db *sql.DB) error {
	return closeDB(db)
}

// createTable creates a table in sqlite3 based on the create table string parameter
func createTable(createStmt string, db *sql.DB) error {
	_, err := db.Exec(createStmt)
	if err != nil {
		return CreateTableError.Wrap(err)
	}
	return nil
}

// SetServerDB public function for a server
func SetServerDB(filepath string) (*sql.DB, error) {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		db, err := startDB(filepath)
		if err != nil {
			return nil, err
		}
		return nil, createTable(createStmt, db)
	}
	db, err := startDB(filepath)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// insertRows inserts the slice of reputation row structs based on the insert string
func insertRows(db *sql.DB, rows []nodeReputationRecord, insertString string) error {
	tx, err := db.Begin()
	if err != nil {
		return InsertError.Wrap(err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.Prepare(insertString)
	if err != nil {
		return InsertError.Wrap(err)
	}
	defer insertStmt.Close()

	for _, row := range rows {
		_, err = insertStmt.Exec(
			row.source,
			row.nodeName,
			row.uptime,
			row.auditSuccess,
			row.auditFail,
			row.latency,
			row.amountOfDataStored,
			row.falseClaims,
			row.shardsModified,
		)
		if err != nil {
			return InsertError.Wrap(err)
		}
	}
	return tx.Commit()
}

// base type for the filter operation for a sql where clause
type whereOpt string

// coproduct/sum type for the generation of the sql string statement
const (
	equal        whereOpt = "="
	greater      whereOpt = ">"
	greaterEqual whereOpt = ">="
	less         whereOpt = "<"
	lessEqual    whereOpt = "<="
	notEqual     whereOpt = "!="
)

// toString is a method to convert the sum type to a string for the sql string
func (opt whereOpt) toString() string {
	res := ""
	switch opt {
	case equal:
		res = "="
	case greater:
		res = ">"
	case greaterEqual:
		res = ">="
	case less:
		res = "<"
	case lessEqual:
		res = "<="
	case notEqual:
		res = "!="
	}

	return res
}

// genWhereStatement is a function that makes a sql string with a single where clause
func genWhereStatement(selectAll string, col column, opt whereOpt, value string) string {
	where := " WHERE"
	operand := opt.toString()

	switch col {
	case sourceColumn:
		where = where + " source" + operand + " '" + value + "'"
	case nodeNameColumn:
		where = where + " node_name" + operand + " '" + value + "'"
	case timestampColumn:
		where = where + " timestamp" + operand + " STRFTIME('%Y-%m-%d %H:%M:%f'," + value + ")"
	case uptimeColumn:
		where = where + " uptime" + operand + " " + value
	case auditSuccessColumn:
		where = where + " audit_succes" + operand + " " + value
	case auditFailColumn:
		where = where + " audit_fail" + operand + " " + value
	case latencyColumn:
		where = where + " latency" + operand + " " + value
	case amountOfDataStoredColumn:
		where = where + " amount_of_data_stored" + operand + " " + value
	case falseClaimsColumn:
		where = where + " false_claims" + operand + " " + value
	case shardsModifiedColumn:
		where = where + " shards_modified" + operand + " " + value

	default:
		where = ""
	}

	return selectAll + where
}

// iterOnDBRows iterate on rows in the database to transform into slice of nodeReputationRecord
func iterOnDBRows(rows *sql.Rows) ([]nodeReputationRecord, error) {
	var res []nodeReputationRecord

	for rows.Next() {
		var row nodeReputationRecord

		err := rows.Scan(
			&row.source,
			&row.nodeName,
			&row.timestamp,
			&row.uptime,
			&row.auditSuccess,
			&row.auditFail,
			&row.latency,
			&row.amountOfDataStored,
			&row.falseClaims,
			&row.shardsModified,
		)
		if err != nil {
			return nil, IterError.Wrap(err)
		}

		res = append(res, row)
	}

	return res, nil
}

// getNodeReputationRecords function that returns a slice of reputation rows based on the query string
func getNodeReputationRecords(db *sql.DB, selectString string) ([]nodeReputationRecord, error) {
	rows, err := db.Query(selectString)
	if err != nil {
		return nil, SelectError.Wrap(err)
	}
	defer rows.Close()

	res, err := iterOnDBRows(rows)
	if err != nil {
		return nil, SelectError.Wrap(err)
	}

	err = rows.Err()
	if err != nil {
		return nil, SelectError.Wrap(err)
	}

	return res, nil
}

/*
  pruneNodeReputationRecords is very destructive!
  this function is used to make a snapshot of the current node
  it removes the data that is older than the node passed in
*/
func pruneNodeReputationRecords(db *sql.DB, recordToKeep nodeReputationRecord, deleteString string) error {
	tx, err := db.Begin()
	if err != nil {
		return DeleteError.Wrap(err)
	}
	defer tx.Rollback()

	deleteStmt, err := tx.Prepare(deleteString)
	if err != nil {
		return DeleteError.Wrap(err)
	}
	defer deleteStmt.Close()

	_, err = deleteStmt.Exec(
		recordToKeep.nodeName,
		recordToKeep.source,
		recordToKeep.nodeName,
		recordToKeep.timestamp,
		recordToKeep.uptime,
		recordToKeep.auditSuccess,
		recordToKeep.auditFail,
		recordToKeep.latency,
		recordToKeep.amountOfDataStored,
		recordToKeep.falseClaims,
		recordToKeep.shardsModified,
	)
	if err != nil {
		return DeleteError.Wrap(err)
	}
	return tx.Commit()
}

// closeDB close sqlite3
func closeDB(db *sql.DB) error {
	return db.Close()
}
