package reputation

import (
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
		name,
		"",
		seed,
		seed,
		seed,
		seed,
		seed,
		seed,
		0,
	}
}

// create a slice of reputation row with random data with the number of rows based on the max row parameter
func createRandRows(maxRows int) []RepRow {
	var res []RepRow

	for i := 0; i < maxRows+1; i++ {
		uid := uuid.New()

		row := createNamedRow(i, uid.String())

		res = append(res, row)
	}

	return res
}

// creates a slice of reputation row structs filled with random data with the name column from the names slice
func createNamedRandRows(names []string) []RepRow {
	var res []RepRow

	for idx, name := range names {
		row := createNamedRow(idx, name)

		res = append(res, row)
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

	defer insertStmt.Close()
}

// side effect function that prints the rows from the query string
func selectFromDB(db *sql.DB, selectString string) {
	rows, err := db.Query(selectString)
	if err != nil {
		log.Fatal(err)
	}

	for rows.Next() {
		var name string
		var timestamp string
		var uptime int
		var auditSuccess int
		var auditFail int
		var latency int
		var amountOfDataStored int
		var falseClaims int
		var shardsModified int

		err = rows.Scan(
			&name,
			&timestamp,
			&uptime,
			&auditSuccess,
			&auditFail,
			&latency,
			&amountOfDataStored,
			&falseClaims,
			&shardsModified)
		if err != nil {
			log.Fatal(err)
		}

		// side effect
		fmt.Println(name,
			timestamp,
			uptime,
			auditSuccess,
			auditFail,
			latency,
			amountOfDataStored,
			falseClaims,
			shardsModified)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()
}

// function that returns a slice of reputation rows based on the query string
func getRepRows(db *sql.DB, selectString string) []RepRow {
	var res []RepRow

	rows, err := db.Query(selectString)
	if err != nil {
		log.Fatal(err)
	}

	for rows.Next() {
		var name string
		var timestamp string
		var uptime int
		var auditSuccess int
		var auditFail int
		var latency int
		var amountOfDataStored int
		var falseClaims int
		var shardsModified int

		err = rows.Scan(
			&name,
			&timestamp,
			&uptime,
			&auditSuccess,
			&auditFail,
			&latency,
			&amountOfDataStored,
			&falseClaims,
			&shardsModified,
		)
		if err != nil {
			log.Fatal(err)
		}

		currentRow := RepRow{
			name,
			timestamp,
			uptime,
			auditSuccess,
			auditFail,
			latency,
			amountOfDataStored,
			falseClaims,
			shardsModified,
		}

		res = append(res, currentRow)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	return res
}

// close sqlite3
func cleanUpDB(db *sql.DB) {
	defer db.Close()
}

// finds the ration of audit success from the success and failure fields of a given reputaion row struct
func (row RepRow) auditSuccessRatio() float64 {
	return float64(row.auditSuccess) / float64(row.auditSuccess+row.auditFail)
}

// naive formula for obtaining repuataion
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

// compares reputation rows and returns the greater reputation of the two
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

	for rows.Next() {
		var name string
		var timestamp string
		var uptime int
		var auditSuccess int
		var auditFail int
		var latency int
		var amountOfDataStored int
		var falseClaims int
		var shardsModified int

		err = rows.Scan(
			&name,
			&timestamp,
			&uptime,
			&auditSuccess,
			&auditFail,
			&latency,
			&amountOfDataStored,
			&falseClaims,
			&shardsModified)
		if err != nil {
			log.Fatal(err)
		}

		currentRow := RepRow{
			name,
			timestamp,
			uptime,
			auditSuccess,
			auditFail,
			latency,
			amountOfDataStored,
			falseClaims,
			shardsModified}

		bestRep = bestRep.greaterRep(currentRow)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

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

// performs an operation that is passed to it on a value with the scalar
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
	case "uptime":
		row.uptime = performOp(op, row.uptime, scalar)
	case "auditSuccess":
		row.auditSuccess = performOp(op, row.auditSuccess, scalar)
	case "auditFail":
		row.auditFail = performOp(op, row.auditFail, scalar)
	case "latency":
		row.latency = performOp(op, row.latency, scalar)
	case "amountOfDataStored":
		row.amountOfDataStored = performOp(op, row.amountOfDataStored, scalar)
	case "falseClaims":
		row.falseClaims = performOp(op, row.falseClaims, scalar)
	case "shardsModified":
		row.shardsModified = performOp(op, row.shardsModified, scalar)
	}

	return row
}

// this is more like map because the slice is the functor
func repRowMorphism(rows []RepRow, col column, op mutOp, scalar int) []RepRow {
	var res []RepRow

	for _, row := range rows {
		res = append(res, row.morphism(col, op, scalar))
	}

	return res
}

// NewReputationRow this is the apply function for the reputation row struct
func NewReputationRow(name string) RepRow {
	res := RepRow{name, "", 0, 0, 0, 0, 0, 0, 0}
	return res
}
