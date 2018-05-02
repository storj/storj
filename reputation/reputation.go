// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
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

// NodeReputationRecord is the Data type for Rows in Reputation table
type NodeReputationRecord struct {
	name               string
	timestamp          string
	uptime             int
	auditSuccess       int
	auditFail          int
	latency            int
	amountOfDataStored int
	falseClaims        int
	shardsModified     int
}

// startDB starts a sqlite3 database from the file path parameter
func startDB(filePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		log.Printf("%q\n", err)
		return nil, StartDBError.Wrap(err)
	}

	return db, nil
}

// createTable creates a table in sqlite3 based on the create table string parameter
func createTable(createStmt string, db *sql.DB) error {
	_, err := db.Exec(createStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, createStmt)
		return InsertError.Wrap(err)
	}
	return nil
}

// createNamedRow creates a reputation row struct with a name field base on the name parameter
func createNamedRow(seed int, name string) NodeReputationRecord {
	return NodeReputationRecord{
		name:               name,
		timestamp:          "",
		uptime:             seed,
		auditSuccess:       seed,
		auditFail:          seed,
		latency:            seed,
		amountOfDataStored: seed,
		falseClaims:        seed,
		shardsModified:     0,
	}
}

// createRandRows create a slice of reputation row with random data with the number of rows based on the max row parameter
func createRandRows(numRows int) []NodeReputationRecord {
	res := make([]NodeReputationRecord, 0, numRows)

	for i := 0; i <= numRows; i++ {
		res = append(res, createNamedRow(i, uuid.New().String()))
	}

	return res
}

// createNameRandRows creates a slice of reputation row structs filled with random data with the name column from the names slice
func createNamedRandRows(names []string) []NodeReputationRecord {
	var res []NodeReputationRecord

	for idx, name := range names {
		res = append(res, createNamedRow(idx, name))
	}

	return res
}

// insertRows inserts the slice of reputation row structs based on the insert string
func insertRows(db *sql.DB, rows []NodeReputationRecord, insertString string) error {
	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		log.Printf("%q: %s\n", err, insertString)
		return InsertError.Wrap(err)
	}

	insertStmt, err := tx.Prepare(insertString)
	if err != nil {
		log.Printf("%q: %s\n", err, insertString)
		return InsertError.Wrap(err)
	}
	defer insertStmt.Close()

	for _, row := range rows {
		_, err = insertStmt.Exec(
			row.name,
			row.uptime,
			row.auditSuccess,
			row.auditFail,
			row.latency,
			row.amountOfDataStored,
			row.falseClaims,
			row.shardsModified,
		)
		if err != nil {
			log.Printf("%q: %s\n", err, insertString)
			return InsertError.Wrap(err)
		}
	}
	tx.Commit()

	return nil
}

// selectFromDB side effect function that prints the rows from the query string
func selectFromDB(db *sql.DB, selectString string) error {
	rows, err := db.Query(selectString)
	if err != nil {
		log.Printf("%q: %s\n", err, selectString)
		return SelectError.Wrap(err)
	}
	defer rows.Close()

	transformedRows, err := iterOnDBRows(rows)
	if err != nil {
		log.Printf("%q: %s\n", err, selectString)
		return SelectError.Wrap(err)
	}

	for _, row := range transformedRows {
		// side effect
		fmt.Println(row)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("%q: %s\n", err, selectString)
		return SelectError.Wrap(err)
	}

	return nil
}

// iterOnDBRows iterate on rows in the database to transform into slice of NodeReputationRecord
func iterOnDBRows(rows *sql.Rows) ([]NodeReputationRecord, error) {
	var res []NodeReputationRecord

	for rows.Next() {
		var row NodeReputationRecord

		err := rows.Scan(
			&row.name,
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
			log.Printf("%q\n", err)
			return nil, IterError.Wrap(err)
		}

		res = append(res, row)
	}

	return res, nil
}

// getNodeReputationRecords function that returns a slice of reputation rows based on the query string
func getNodeReputationRecords(db *sql.DB, selectString string) ([]NodeReputationRecord, error) {
	rows, err := db.Query(selectString)
	if err != nil {
		log.Printf("%q: %s\n", err, selectString)
		return nil, SelectError.Wrap(err)
	}
	defer rows.Close()

	res, err := iterOnDBRows(rows)
	if err != nil {
		log.Printf("%q: %s\n", err, selectString)
		return nil, SelectError.Wrap(err)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("%q: %s\n", err, selectString)
		return nil, SelectError.Wrap(err)
	}

	return res, nil
}

// func genDeleteStmt(deleteString string, recordToKeep NodeReputationRecord) string {
// 	return fmt.Sprintf(deleteString,
// 		recordToKeep.name,
// 		recordToKeep.timestamp,
// 		recordToKeep.uptime,
// 		recordToKeep.auditSuccess,
// 		recordToKeep.auditFail,
// 		recordToKeep.latency,
// 		recordToKeep.amountOfDataStored,
// 		recordToKeep.falseClaims,
// 		recordToKeep.shardsModified,
// 	)
// }

/*
  pruneNodeReputationRecords is very destructive!
  this function is used to make a snapshot of the current node
  it removes the data that is older than the node passed in
*/
func pruneNodeReputationRecords(db *sql.DB, recordToKeep NodeReputationRecord, deleteString string) error {
	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		log.Printf("%q: %s\n", err, deleteString)
		return DeleteError.Wrap(err)
	}

	deleteStmt, err := tx.Prepare(deleteString)
	if err != nil {
		log.Printf("%q: %s\n", err, deleteString)
		return DeleteError.Wrap(err)
	}
	defer deleteStmt.Close()

	_, err = deleteStmt.Exec(
		recordToKeep.name,
		recordToKeep.name,
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
		log.Printf("%q: %v\n", err, deleteStmt)
		return DeleteError.Wrap(err)
	}
	tx.Commit()

	return nil
}

// cleanUpDB close sqlite3
func cleanUpDB(db *sql.DB) {
	db.Close()
}

// auditSuccessRatio finds the ratio of audit success from the success and failure fields of a given reputaion row struct
func (row NodeReputationRecord) auditSuccessRatio() float64 {
	return float64(row.auditSuccess) / float64(row.auditSuccess+row.auditFail)
}

/*
  naiveRep is naive formula for obtaining a repuataion score (scalar)
  this method favors uptime, hence being multiplied by 100
  and nullifys a score if there is a case of data modification
*/
func (row NodeReputationRecord) naiveRep() float64 {
	var mutator int

	if row.shardsModified > 0 {
		mutator = 0
	} else {
		mutator = 1
	}

	return (float64(row.uptime*100) +
		row.auditSuccessRatio() +
		float64(row.latency) +
		float64(row.amountOfDataStored) -
		float64(row.falseClaims)) * float64(mutator)
}

/*
  greaterRep compares reputation rows and returns the greater reputation of the two
  this method condsiders a reputation greater:
  if the time is more recent it is greater
  else use naive reputation method
*/
func (row NodeReputationRecord) greaterRep(other NodeReputationRecord) NodeReputationRecord {
	myRep := row.naiveRep()
	otherRep := other.naiveRep()
	myTime := row.timestamp
	otherTime := other.timestamp
	myName := row.name
	otherName := other.name

	var res NodeReputationRecord

	switch {
	case myTime < otherTime && myName == otherName:
		res = other
	case myTime > otherTime && myName == otherName:
		res = row
	case myRep > otherRep:
		res = row
	default:
		res = other
	}

	return res
}

// naiveReputation finds the naive reputation of the resulting rows from the query string
func naiveReputation(db *sql.DB, queryString string) (NodeReputationRecord, error) {
	bestRep := NodeReputationRecord{"identity", "", 0, 0, 0, 0, 0, 0, 0}

	rows, err := db.Query(queryString)
	if err != nil {
		log.Printf("%q: %s\n", err, queryString)
		return bestRep, SelectError.Wrap(err)
	}
	defer rows.Close()

	transformedRows, err := iterOnDBRows(rows)
	if err != nil {
		log.Printf("%q: %s\n", err, queryString)
		return bestRep, SelectError.Wrap(err)
	}

	for _, row := range transformedRows {
		bestRep = bestRep.greaterRep(row)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("%q: %s\n", err, queryString)
		return bestRep, SelectError.Wrap(err)
	}

	return bestRep, nil
}

/*
  endian method hot encodes the two NodeReputationRecord structs
  desired values are set to a one, other values are set to zeros
  then compares and returns the largest
  order is as follows:
  timestamp, most recent values of rows with the same name equals a one
  shardsModified, if any value other than zero is found a zero is needed
  falseClaims, more false claims equal a zero
  auditSuccessRatio, higher ratio equals a one
  uptime, higher uptime equals a one
  latency, lower latency equals a one
  amountOfDataStored, more data equals a one
*/
func (row NodeReputationRecord) endian(other NodeReputationRecord) NodeReputationRecord {
	var rowEndian bytes.Buffer
	var otherEndian bytes.Buffer

	switch {
	case row.timestamp > other.timestamp && row.name == other.name:
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.timestamp < other.timestamp && row.name == other.name:
		rowEndian.WriteString("0")
		otherEndian.WriteString("1")
	default:
		rowEndian.WriteString("0")
		otherEndian.WriteString("0")
	}

	if row.shardsModified > 0 {
		rowEndian.WriteString("0")
	} else {
		rowEndian.WriteString("1")
	}
	if other.shardsModified > 0 {
		otherEndian.WriteString("0")
	} else {
		otherEndian.WriteString("1")
	}

	switch {
	case row.falseClaims < other.falseClaims:
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.falseClaims > other.falseClaims:
		rowEndian.WriteString("0")
		otherEndian.WriteString("1")
	default:
		rowEndian.WriteString("0")
		otherEndian.WriteString("0")
	}

	switch {
	case row.auditSuccessRatio() > other.auditSuccessRatio():
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.auditSuccessRatio() < other.auditSuccessRatio():
		rowEndian.WriteString("0")
		otherEndian.WriteString("1")
	default:
		rowEndian.WriteString("0")
		otherEndian.WriteString("0")
	}

	switch {
	case row.uptime > other.uptime:
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.uptime < other.uptime:
		rowEndian.WriteString("0")
		otherEndian.WriteString("1")
	default:
		rowEndian.WriteString("0")
		otherEndian.WriteString("0")
	}

	switch {
	case row.latency < other.latency:
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.latency > other.latency:
		rowEndian.WriteString("0")
		otherEndian.WriteString("1")
	default:
		rowEndian.WriteString("0")
		otherEndian.WriteString("0")
	}

	switch {
	case row.amountOfDataStored > other.amountOfDataStored:
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.amountOfDataStored < other.amountOfDataStored:
		rowEndian.WriteString("0")
		otherEndian.WriteString("1")
	default:
		rowEndian.WriteString("0")
		otherEndian.WriteString("0")
	}

	var res NodeReputationRecord

	if rowEndian.String() > otherEndian.String() {
		res = row
	} else {
		res = other
	}

	fmt.Printf("endian: %v, me: %v\n", rowEndian.String(), row.name)
	fmt.Printf("endian: %v, other: %v\n", otherEndian.String(), other.name)

	fmt.Printf("WINNER: %v\n\n", res)

	return res
}

// endianReputation based on the most significant fields of NodeReputationRecord
func endianReputation(db *sql.DB, queryString string) (NodeReputationRecord, error) {
	bestRep := NodeReputationRecord{"identity", "", 0, 0, 0, 0, 0, 0, 0}

	rows, err := db.Query(queryString)
	if err != nil {
		log.Printf("%q: %s\n", err, queryString)
		return bestRep, SelectError.Wrap(err)
	}
	defer rows.Close()

	transformedRows, err := iterOnDBRows(rows)
	if err != nil {
		log.Printf("%q: %s\n", err, queryString)
		return bestRep, SelectError.Wrap(err)
	}

	for _, row := range transformedRows {
		bestRep = bestRep.endian(row)
	}

	err = rows.Err()
	if err != nil {
		log.Printf("%q: %s\n", err, queryString)
		return bestRep, SelectError.Wrap(err)
	}

	return bestRep, nil
}

type mutOp string

// coproduct/sum type for the mutation operation type
const (
	increment mutOp = "increment"
	decrement mutOp = "decrement"
	overWrite mutOp = "overWrite"
)

// performOp performs an operation that is passed to it on a value with the scalar
func performOp(op mutOp, value int, scalar int) int {
	switch op {
	case increment:
		value = value + scalar
	case decrement:
		value = value - scalar
	case overWrite:
		value = scalar
	}

	return value
}

type column string

// coproduct/sum type for the column type
const (
	uptimeColumn             column = "uptime"
	auditSuccessColumn       column = "auditSuccess"
	auditFailColumn          column = "auditFail"
	latencyColumn            column = "latency"
	amountOfDataStoredColumn column = "amountOfDataStored"
	falseClaimsColumn        column = "falseClaims"
	shardsModifiedColumn     column = "shardsModified"
)

// morphism is the name of this function becuse it does not directly change the NodeReputationRecord (more map/functor like)
func (row NodeReputationRecord) morphism(col column, op mutOp, scalar int) NodeReputationRecord {
	switch col {
	case uptimeColumn:
		row.uptime = performOp(op, row.uptime, scalar)
	case auditSuccessColumn:
		row.auditSuccess = performOp(op, row.auditSuccess, scalar)
	case auditFailColumn:
		row.auditFail = performOp(op, row.auditFail, scalar)
	case latencyColumn:
		row.latency = performOp(op, row.latency, scalar)
	case amountOfDataStoredColumn:
		row.amountOfDataStored = performOp(op, row.amountOfDataStored, scalar)
	case falseClaimsColumn:
		row.falseClaims = performOp(op, row.falseClaims, scalar)
	case shardsModifiedColumn:
		row.shardsModified = performOp(op, row.shardsModified, scalar)
	}

	return row
}

// NodeReputationRecordMorphism this is more like fmap because the slice is the functor, returns a new NodeReputationRecord slice
func NodeReputationRecordMorphism(rows []NodeReputationRecord, col column, op mutOp, scalar int) []NodeReputationRecord {
	var res []NodeReputationRecord

	for _, row := range rows {
		res = append(res, row.morphism(col, op, scalar))
	}

	return res
}

// NewReputationRow this is the apply function for the reputation row struct, returns a new NodeReputationRecord
func NewReputationRow(name string) NodeReputationRecord {
	return NodeReputationRecord{name, "", 0, 0, 0, 0, 0, 0, 0}
}
