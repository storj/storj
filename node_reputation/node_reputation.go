// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"bytes"
	"database/sql"

	// import of sqlite3 for side effects
	_ "github.com/mattn/go-sqlite3"
)

// auditSuccessRatio finds the ratio of audit success from the success and failure fields of a given reputaion row struct
func (row nodeReputationRecord) auditSuccessRatio() float64 {
	return float64(row.auditSuccess) / float64(row.auditSuccess+row.auditFail)
}

/*
  naiveRep is naive formula for obtaining a repuataion score (scalar)
  this method favors uptime, hence being multiplied by 100
  and nullifys a score if there is a case of data modification
*/
func (row nodeReputationRecord) naiveRep() float64 {
	var mutator int64

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
func (row nodeReputationRecord) greaterRep(other nodeReputationRecord) nodeReputationRecord {
	myRep := row.naiveRep()
	otherRep := other.naiveRep()
	myTime := row.timestamp
	otherTime := other.timestamp
	myName := row.nodeName
	otherName := other.nodeName

	var res nodeReputationRecord

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
func naiveReputation(db *sql.DB, queryString string) (nodeReputationRecord, error) {
	bestRep := nodeReputationRecord{"self", "identity", "", 0, 0, 0, 0, 0, 0, 0}

	rows, err := db.Query(queryString)
	if err != nil {
		return bestRep, SelectError.Wrap(err)
	}
	defer rows.Close()

	transformedRows, err := iterOnDBRows(rows)
	if err != nil {
		return bestRep, SelectError.Wrap(err)
	}

	for _, row := range transformedRows {
		bestRep = bestRep.greaterRep(row)
	}

	err = rows.Err()
	if err != nil {
		return bestRep, SelectError.Wrap(err)
	}

	return bestRep, nil
}

/*
  endian method hot encodes the two nodeReputationRecord structs
  desired values are set to a one, other values are set to zeros
  then compares and returns the largest
  order of evaluation is ordered from most significant column (first)
  to the least significant column (last position in the slice)
*/
func (row nodeReputationRecord) endian(other nodeReputationRecord, orderOfEval []column) nodeReputationRecord {
	var rowEndian bytes.Buffer
	var otherEndian bytes.Buffer

	for _, order := range orderOfEval {
		switch order {
		case timestampColumn:
			switch {
			case row.timestamp > other.timestamp && row.nodeName == other.nodeName:
				rowEndian.WriteString("1")
				otherEndian.WriteString("0")
			case row.timestamp < other.timestamp && row.nodeName == other.nodeName:
				rowEndian.WriteString("0")
				otherEndian.WriteString("1")
			default:
				rowEndian.WriteString("0")
				otherEndian.WriteString("0")
			}

		case shardsModifiedColumn:
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

		case falseClaimsColumn:
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

		case auditSuccessColumn:
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

		case uptimeColumn:
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

		case latencyColumn:
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

		case amountOfDataStoredColumn:
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
		}
	}

	var res nodeReputationRecord

	if rowEndian.String() > otherEndian.String() {
		res = row
	} else {
		res = other
	}

	//side effects
	/*
		fmt.Printf("endian: %v, me: %v\n", rowEndian.String(), row.nodeName)
		fmt.Printf("endian: %v, other: %v\n", otherEndian.String(), other.nodeName)

		fmt.Printf("WINNER: %v\n\n", res)
	*/
	return res
}

// serde converts private record to public reputation record
func (row nodeReputationRecord) serde() NodeReputationRecord {
	return NodeReputationRecord{
		Source:             row.source,
		NodeName:           row.nodeName,
		Timestamp:          row.timestamp,
		Uptime:             row.uptime,
		AuditSuccess:       row.auditSuccess,
		AuditFail:          row.auditFail,
		Latency:            row.latency,
		AmountOfDataStored: row.amountOfDataStored,
		FalseClaims:        row.falseClaims,
		ShardsModified:     row.shardsModified,
	}
}

/*
  endianReputation based on the most significant fields of nodeReputationRecord
  order is as follows:
  timestamp, most recent values of rows with the same name equals a one
  shardsModified, if any value other than zero is found a zero is needed
  falseClaims, more false claims equal a zero
  auditSuccessRatio, higher ratio equals a one
  uptime, higher uptime equals a one
  latency, lower latency equals a one
  amountOfDataStored, more data equals a one
*/
func endianReputation(db *sql.DB, queryString string) (nodeReputationRecord, error) {
	bestRep := nodeReputationRecord{"self", "identity", "", 0, 0, 0, 0, 0, 0, 0}

	rows, err := db.Query(queryString)
	if err != nil {
		return bestRep, SelectError.Wrap(err)
	}
	defer rows.Close()

	transformedRows, err := iterOnDBRows(rows)
	if err != nil {
		return bestRep, SelectError.Wrap(err)
	}

	order := []column{
		timestampColumn,
		shardsModifiedColumn,
		falseClaimsColumn,
		auditSuccessColumn,
		uptimeColumn,
		latencyColumn,
		amountOfDataStoredColumn,
	}

	for _, row := range transformedRows {
		bestRep = bestRep.endian(row, order)
	}

	err = rows.Err()
	if err != nil {
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
func performOp(op mutOp, value int64, scalar int64) int64 {
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

// morphism is the name of this function becuse it does not directly change the nodeReputationRecord (more map/functor like)
func (row nodeReputationRecord) morphism(col column, op mutOp, scalar int64) nodeReputationRecord {
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

// nodeReputationRecordMorphism this is more like fmap because the slice is the functor, returns a new nodeReputationRecord slice
func nodeReputationRecordMorphism(rows []nodeReputationRecord, col column, op mutOp, scalar int64) []nodeReputationRecord {
	var res []nodeReputationRecord

	for _, row := range rows {
		res = append(res, row.morphism(col, op, scalar))
	}

	return res
}

// newReputationRow this is the apply function for the reputation row struct, returns a new nodeReputationRecord
func newReputationRow(source string, name string) nodeReputationRecord {
	return nodeReputationRecord{source, name, "", 0, 0, 0, 0, 0, 0, 0}
}
