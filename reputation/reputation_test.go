// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package reputation

import (
	"fmt"
	"os"
	"testing"
)

func TestCreateTable(t *testing.T) {
	db, _ := startDB("./TestCreateTable.db")

	createStmt := `
	CREATE table foo (id interger not null primary key, name text);
	`
	createTable(createStmt, db)

	os.Remove("./TestCreateTable.db")
}

func TestRowInsertAndQuery(t *testing.T) {
	db, _ := startDB("./TestRowInsertAndQuery.db")

	createStmt := `
	CREATE table reputation (
		name text not null primary key,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')), 
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	selectStmt := `SELECT 
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation
	LIMIT 2`

	createTable(createStmt, db)

	insertRows(db, createRandRows(100), insertStmt)

	selectFromDB(db, selectStmt)

	cleanUpDB(db)

	os.Remove("./TestRowInsertAndQuery.db")
}

func TestFindNode(t *testing.T) {
	db, _ := startDB("./TestFindNode.db")

	createStmt := `
	CREATE table reputation (
		name text not null,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')), 
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger,
	PRIMARY KEY(name, timestamp)
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	selectStmt := `SELECT 
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation`

	createTable(createStmt, db)

	rows := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := naiveReputation(db, selectStmt)

	if bestRep.name != "Carol" {
		t.Error(
			"expected Carol got", bestRep.name,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	cleanUpDB(db)

	os.Remove("./TestFindNode.db")
}

func TestUpdateNode(t *testing.T) {
	db, _ := startDB("./TestUpdateNode.db")

	createStmt := `
	CREATE table reputation (
		name text not null,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')), 
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger,
	PRIMARY KEY(name, timestamp)
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	selectStmt := `SELECT 
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation`

	createTable(createStmt, db)

	rows := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := naiveReputation(db, selectStmt)

	if bestRep.name != "Carol" {
		t.Error(
			"expected Carol got", bestRep.name,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	rows1 := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 6, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 11, 20, 0, 10, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 1, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 16, 10, 0, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 6, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows1, insertStmt)
	bestRep1, _ := naiveReputation(db, selectStmt)

	if bestRep1.name != "Dave" {
		t.Error(
			"expected Dave got", bestRep1.name,
			"with a reputation of", bestRep1.naiveRep(),
		)
	}

	cleanUpDB(db)

	os.Remove("./TestUpdateNode.db")
}

func TestFindNodeNoFail(t *testing.T) {
	db, _ := startDB("./TestFindNodeNoFail.db")

	createStmt := `
	CREATE table reputation (
		name text not null,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')), 
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger,
	PRIMARY KEY(name, timestamp)
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	noFailStmt := `SELECT 
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation
	WHERE audit_fail == 0 
	  AND amount_of_data_stored <= 200`

	createTable(createStmt, db)

	rows := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := naiveReputation(db, noFailStmt)

	if bestRep.name != "Bob" {
		t.Error(
			"expected Bob got", bestRep.name,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	cleanUpDB(db)

	os.Remove("./TestFindNodeNoFail.db")
}

func TestMorphismOfRow(t *testing.T) {
	alice := NewReputationRow("Alice")
	alice = alice.morphism(uptimeColumn, overWrite, 5)
	// col needed else NaN
	alice = alice.morphism(auditSuccessColumn, overWrite, 10)

	op1 := alice.morphism(uptimeColumn, increment, 1)
	if op1.uptime != 6 {
		t.Error(
			"expected uptime as 6 got", op1.uptime,
		)
	}

	op2 := alice.morphism(uptimeColumn, decrement, 2)
	if op2.uptime != 3 {
		t.Error(
			"expected uptime as 4 got", op2.uptime,
		)
	}

	op3 := alice.morphism(uptimeColumn, overWrite, 10)
	if op3.uptime != 10 {
		t.Error(
			"expected uptime as 4 got", op3.uptime,
		)
	}

	op4 := alice.morphism(shardsModifiedColumn, overWrite, 1)
	if op4.naiveRep() != float64(0) {
		t.Error(
			"expected reputation of 0 got", op4.naiveRep(),
		)
	}

}

func TestMorphismAndInsertOfRow(t *testing.T) {
	db, _ := startDB("./TestMorphismAndInsertOfRow.db")

	createStmt := ` CREATE table reputation (
		name text not null,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')),
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger,
	PRIMARY KEY(name, timestamp)
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	selectAliceStmt := `SELECT
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation
	WHERE name = 'Alice'`

	createTable(createStmt, db)

	rows := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	aliceRep, _ := naiveReputation(db, selectAliceStmt)

	aliceNaiveRep := fmt.Sprintf("%.2f", aliceRep.naiveRep())

	if aliceNaiveRep != "605.67" {
		t.Error(
			"expected Alice got", aliceRep.name,
			"with a reputation of 605.67", aliceNaiveRep,
		)
	}

	aliceNewRep, _ := getNodeReputationRecords(db, selectAliceStmt)
	aliceNewRow := NodeReputationRecordMorphism(aliceNewRep, shardsModifiedColumn, increment, 2)

	insertRows(db, aliceNewRow, insertStmt)

	aliceRows, _ := getNodeReputationRecords(db, selectAliceStmt)
	if len(aliceRows) != 2 {
		t.Error(
			"expected 2 rows in the db got", aliceRows,
		)
	}

	morphRep, _ := naiveReputation(db, selectAliceStmt)

	if morphRep.naiveRep() != float64(0) {
		t.Error(
			"expected reputation of 0 for Alice got", morphRep.naiveRep(),
			"expected Alice with shards modified greater that 0", morphRep,
		)
	}

	cleanUpDB(db)

	os.Remove("./TestMorphismAndInsertOfRow.db")
}

func TestEndianMorphismAndInsertOfRow(t *testing.T) {
	db, _ := startDB("./TestEndianMorphismAndInsertOfRow.db")

	createStmt := ` CREATE table reputation (
		name text not null,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')),
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger,
	PRIMARY KEY(name, timestamp)
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	selectAllStmt := `SELECT
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation`

	selectAliceStmt := `SELECT
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation
	WHERE name = 'Alice'`

	createTable(createStmt, db)

	rows := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 10, 10, 5, 1, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 5, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 15, 10, 5, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := endianReputation(db, selectAllStmt)

	if bestRep.name != "Dave" {
		t.Error(
			"expected Dave got", bestRep.name,
		)
	}

	aliceNewRep, _ := getNodeReputationRecords(db, selectAliceStmt)
	aliceNewRow := NodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 20)

	insertRows(db, aliceNewRow, insertStmt)

	newAndOldRows, _ := getNodeReputationRecords(db, selectAllStmt)
	if len(newAndOldRows) != 6 {
		t.Error(
			"expected 6 rows in the db got", newAndOldRows,
		)
	}

	morphRep, _ := endianReputation(db, selectAllStmt)

	if morphRep.name != "Alice" {
		t.Error(
			"expected Alice got", morphRep.name,
		)
	}

	cleanUpDB(db)

	os.Remove("./TestEndianMorphismAndInsertOfRow.db")
}

func TestEndianPrune(t *testing.T) {
	db, _ := startDB("./TestEndianPrune.db")

	createStmt := ` CREATE table reputation (
		name text not null,
		timestamp timestamp DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')),
		uptime interger,
		audit_success interger,
		audit_fail interger,
		latency interger,
		amount_of_data_stored interger,
		false_claims interger,
		shards_modified interger,
	PRIMARY KEY(name, timestamp)
	);`

	insertStmt := `INSERT into reputation (
		name,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	) values (?, ?, ?, ?, ?, ?, ?, ?);`

	selectAllStmt := `SELECT
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation`

	selectAliceStmt := `SELECT
		name,
		timestamp,
		uptime,
		audit_success,
		audit_fail,
		latency,
		amount_of_data_stored,
		false_claims,
		shards_modified
	FROM reputation
	WHERE name = 'Alice'`

	deletStmt := `DELETE FROM reputation
		WHERE
		name = ?
		AND
		timestamp != ?
		AND
		uptime != ?
		AND
		audit_success != ?
		AND
		audit_fail != ?
		AND
		latency != ?
		AND
		amount_of_data_stored != ?
		AND
		false_claims != ?
		AND
		shards_modified != ?`

	createTable(createStmt, db)

	rows := []NodeReputationRecord{
		NodeReputationRecord{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		NodeReputationRecord{"Bob", "", 10, 10, 5, 1, 100, 0, 0},
		NodeReputationRecord{"Carol", "", 5, 10, 5, 3, 100, 0, 0},
		NodeReputationRecord{"Dave", "", 15, 10, 5, 5, 500, 0, 0},
		NodeReputationRecord{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := endianReputation(db, selectAllStmt)

	if bestRep.name != "Dave" {
		t.Error(
			"expected Dave got", bestRep.name,
		)
	}

	aliceNewRep, _ := getNodeReputationRecords(db, selectAliceStmt)

	aliceNewRow := NodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 10)
	insertRows(db, aliceNewRow, insertStmt)

	aliceNewRow1 := NodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 2)
	insertRows(db, aliceNewRow1, insertStmt)

	aliceNewRow2 := NodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 22)
	insertRows(db, aliceNewRow2, insertStmt)

	aliceNewRow3 := NodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 30)
	insertRows(db, aliceNewRow3, insertStmt)

	newAndOldRows, _ := getNodeReputationRecords(db, selectAllStmt)
	if len(newAndOldRows) != 9 {
		t.Error(
			"expected 9 rows in the db got", newAndOldRows,
		)
	}

	morphRep, _ := endianReputation(db, selectAllStmt)

	if morphRep.name != "Alice" {
		t.Error(
			"expected Alice got", morphRep.name,
		)
	}

	onlyAlices, _ := getNodeReputationRecords(db, selectAliceStmt)
	if len(onlyAlices) != 5 {
		t.Error(
			"expected 5 rows in the db got", onlyAlices,
		)
	}

	bestAlice, _ := endianReputation(db, selectAliceStmt)

	if bestAlice.uptime != 30 {
		t.Error(
			"expected uptime of 30 got", bestAlice.uptime,
		)
	}
	pruneNodeReputationRecords(db, bestAlice, deletStmt)

	onlyAlice, _ := getNodeReputationRecords(db, selectAliceStmt)
	if len(onlyAlice) != 1 {
		t.Error(
			"expected 1 rows in the db got", onlyAlice,
		)
	}

	cleanUpDB(db)

	os.Remove("./TestEndianPrune.db")
}
