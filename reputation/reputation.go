// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
	// import of sqlite3 for side effects
	_ "github.com/mattn/go-sqlite3"
)

// RepRow is the Data type for Rows in Reputation table
type RepRow struct {
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

// starts a sqlite3 database from the file path parameter
func startDB(filePath string) *sql.DB {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

// creates a table in sqlite3 based on the create table string parameter
func createTable(createStmt string, db *sql.DB) {
	_, err := db.Exec(createStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, createStmt)
		return
	}
}

// creates a reputation row struct with a name field base on the name parameter
func createNamedRow(seed int, name string) RepRow {
	return RepRow{
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

// create a slice of reputation row with random data with the number of rows based on the max row parameter
func createRandRows(numRows int) []RepRow {
	res := make([]RepRow, 0, numRows)

	for i := 0; i <= numRows; i++ {
		res = append(res, createNamedRow(i, uuid.New().String()))
	}

	return res
}

// creates a slice of reputation row structs filled with random data with the name column from the names slice
func createNamedRandRows(names []string) []RepRow {
	var res []RepRow

	for idx, name := range names {
		res = append(res, createNamedRow(idx, name))
	}

	return res
}

// inserts the slice of reputation row structs based on the insert string
func insertRows(db *sql.DB, rows []RepRow, insertString string) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	insertStmt, err := tx.Prepare(insertString)
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	}
	tx.Commit()

}

// side effect function that prints the rows from the query string
func selectFromDB(db *sql.DB, selectString string) {
	rows, err := db.Query(selectString)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	transformedRows := iterOnDBRows(rows)

	for _, row := range transformedRows {
		// side effect
		fmt.Println(row)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

}

// iterOnDBRows iterate on rows in the database to transform into slice of RepRow
func iterOnDBRows(rows *sql.Rows) []RepRow {
	var res []RepRow

	for rows.Next() {
		var row RepRow

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
			log.Fatal(err)
		}

		res = append(res, row)
	}

	return res
}

// function that returns a slice of reputation rows based on the query string
func getRepRows(db *sql.DB, selectString string) []RepRow {
	rows, err := db.Query(selectString)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	res := iterOnDBRows(rows)

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	return res
}

// close sqlite3
func cleanUpDB(db *sql.DB) {
	db.Close()
}

// finds the ratio of audit success from the success and failure fields of a given reputaion row struct
func (row RepRow) auditSuccessRatio() float64 {
	return float64(row.auditSuccess) / float64(row.auditSuccess+row.auditFail)
}

/*
  naiveRep is naive formula for obtaining a repuataion score (scalar)
  this method favors uptime, hence being multiplied by 100
  and nullifys a score if there is a case of data modification
*/
func (row RepRow) naiveRep() float64 {
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
  compares reputation rows and returns the greater reputation of the two
  this method condsiders a reputation greater:
  if the time is more recent it is greater
  else use naive reputation method
*/
func (row RepRow) greaterRep(other RepRow) RepRow {
	myRep := row.naiveRep()
	otherRep := other.naiveRep()
	myTime := row.timestamp
	otherTime := other.timestamp

	var res RepRow

	switch {
	case myTime < otherTime:
		res = other
	case myTime > otherTime:
		res = row
	case myRep > otherRep:
		res = row
	default:
		res = other
	}

	return res
}

// finds the naive reputation of the resulting rows from the query string
func naiveReputation(db *sql.DB, queryString string) RepRow {
	bestRep := RepRow{"identity", "", 0, 0, 0, 0, 0, 0, 0}

	rows, err := db.Query(queryString)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	transformedRows := iterOnDBRows(rows)

	for _, row := range transformedRows {
		bestRep = bestRep.greaterRep(row)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	return bestRep
}

/*
  endian method hot encodes the two RepRow structs
  greater values gets a one, ties and other values are zeros
  then compares and returns the largest
  order is as follows:
  timestamp, most recent values are greater
  shardsModified, if any value other than zero is found a zero is needed
  falseClaims, more false claims equal a zero
  auditSuccessRatio, higher ratio equals a one
  uptime, higher uptime equals a one
  latency, lower latency equals a one
  amountOfDataStored, more data equals a one
*/
func (row RepRow) endian(other RepRow) RepRow {
	var rowEndian bytes.Buffer
	var otherEndian bytes.Buffer

	switch {
	case row.timestamp > other.timestamp:
		rowEndian.WriteString("1")
		otherEndian.WriteString("0")
	case row.timestamp < other.timestamp:
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

	var res RepRow

	if rowEndian.String() > otherEndian.String() {
		res = row
	} else {
		res = other
	}

	return res
}

// endianReputation based on the most significant fields of RepRow
func endianReputation(db *sql.DB, queryString string) RepRow {
	bestRep := RepRow{"identity", "", 0, 0, 0, 0, 0, 0, 0}

	rows, err := db.Query(queryString)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	transformedRows := iterOnDBRows(rows)

	for _, row := range transformedRows {
		bestRep = bestRep.endian(row)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	return bestRep
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
	case "increment":
		value = value + scalar
	case "decrement":
		value = value - scalar
	case "overWrite":
		value = scalar
	}

	return value
}

// morphism is the name of this function becuse it does not directly change the RepRow (more map/functor like)
func (row RepRow) morphism(col column, op mutOp, scalar int) RepRow {
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

// repRowMorphism this is more like map because the slice is the functor
func repRowMorphism(rows []RepRow, col column, op mutOp, scalar int) []RepRow {
	var res []RepRow

	for _, row := range rows {
		res = append(res, row.morphism(col, op, scalar))
	}

	return res
}

// NewReputationRow this is the apply function for the reputation row struct
func NewReputationRow(name string) RepRow {
	return RepRow{name, "", 0, 0, 0, 0, 0, 0, 0}
}
