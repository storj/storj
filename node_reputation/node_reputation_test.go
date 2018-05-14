// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"fmt"
	"os"
	"testing"
)

func TestCreateTable(t *testing.T) {
	db, _ := startDB("./TestCreateTable.db")

	createTable(createStmt, db)

	os.Remove("./TestCreateTable.db")
}

func TestRowInsertAndQuery(t *testing.T) {
	db, _ := startDB("./TestRowInsertAndQuery.db")

	createTable(createStmt, db)

	rows := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 5, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 10, 20, 0, 10, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 50, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 15, 10, 0, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := naiveReputation(db, selectAllStmt)

	if bestRep.nodeName != "Carol" {
		t.Error(
			"expected Carol got", bestRep.nodeName,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	cleanUpDB(db)

	os.Remove("./TestRowInsertAndQuery.db")
}

func TestUpdateNode(t *testing.T) {
	db, _ := startDB("./TestUpdateNode.db")

	createTable(createStmt, db)

	rows := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 5, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 10, 20, 0, 10, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 50, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 15, 10, 0, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := naiveReputation(db, selectAllStmt)

	if bestRep.nodeName != "Carol" {
		t.Error(
			"expected Carol got", bestRep.nodeName,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	rows1 := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 6, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 11, 20, 0, 10, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 1, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 16, 10, 0, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 6, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows1, insertStmt)
	bestRep1, _ := naiveReputation(db, selectAllStmt)

	if bestRep1.nodeName != "Dave" {
		t.Error(
			"expected Dave got", bestRep1.nodeName,
			"with a reputation of", bestRep1.naiveRep(),
		)
	}

	cleanUpDB(db)

	os.Remove("./TestUpdateNode.db")
}

func TestFindNodeNoFail(t *testing.T) {
	db, _ := startDB("./TestFindNodeNoFail.db")

	createTable(createStmt, db)

	rows := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 5, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 10, 20, 0, 10, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 50, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 15, 10, 0, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := naiveReputation(db, noFailStmt)

	if bestRep.nodeName != "Bob" {
		t.Error(
			"expected Bob got", bestRep.nodeName,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	cleanUpDB(db)

	os.Remove("./TestFindNodeNoFail.db")
}

func TestMorphismOfRow(t *testing.T) {
	alice := newReputationRow("Test", "Alice")
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

	createTable(createStmt, db)

	rows := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 5, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 10, 20, 0, 10, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 50, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 15, 10, 0, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	selectAliceStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, "Alice")

	aliceRep, _ := naiveReputation(db, selectAliceStmt)

	aliceNaiveRep := fmt.Sprintf("%.2f", aliceRep.naiveRep())

	if aliceNaiveRep != "605.67" {
		t.Error(
			"expected Alice got", aliceRep.nodeName,
			"with a reputation of 605.67", aliceNaiveRep,
		)
	}

	aliceNewRep, _ := getNodeReputationRecords(db, selectAliceStmt)
	aliceNewRow := nodeReputationRecordMorphism(aliceNewRep, shardsModifiedColumn, increment, 2)

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

	createTable(createStmt, db)

	rows := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 5, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 10, 10, 5, 1, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 5, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 15, 10, 5, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := endianReputation(db, selectAllStmt)

	if bestRep.nodeName != "Dave" {
		t.Error(
			"expected Dave got", bestRep.nodeName,
		)
	}

	selectAliceStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, "Alice")
	aliceNewRep, _ := getNodeReputationRecords(db, selectAliceStmt)
	aliceNewRow := nodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 20)

	insertRows(db, aliceNewRow, insertStmt)

	newAndOldRows, _ := getNodeReputationRecords(db, selectAllStmt)
	if len(newAndOldRows) != 6 {
		t.Error(
			"expected 6 rows in the db got", newAndOldRows,
		)
	}

	morphRep, _ := endianReputation(db, selectAllStmt)

	if morphRep.nodeName != "Alice" {
		t.Error(
			"expected Alice got", morphRep.nodeName,
		)
	}

	cleanUpDB(db)

	os.Remove("./TestEndianMorphismAndInsertOfRow.db")
}

func TestEndianPrune(t *testing.T) {
	db, _ := startDB("./TestEndianPrune.db")

	createTable(createStmt, db)

	rows := []nodeReputationRecord{
		nodeReputationRecord{"Test", "Alice", "", 5, 10, 5, 5, 100, 0, 0},
		nodeReputationRecord{"Test", "Bob", "", 10, 10, 5, 1, 100, 0, 0},
		nodeReputationRecord{"Test", "Carol", "", 5, 10, 5, 3, 100, 0, 0},
		nodeReputationRecord{"Test", "Dave", "", 15, 10, 5, 5, 500, 0, 0},
		nodeReputationRecord{"Test", "Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep, _ := endianReputation(db, selectAllStmt)

	if bestRep.nodeName != "Dave" {
		t.Error(
			"expected Dave got", bestRep.nodeName,
		)
	}

	selectAliceStmt := genWhereStatement(selectAllStmt, nodeNameColumn, equal, "Alice")
	aliceNewRep, _ := getNodeReputationRecords(db, selectAliceStmt)

	aliceNewRow := nodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 10)
	insertRows(db, aliceNewRow, insertStmt)

	aliceNewRow1 := nodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 2)
	insertRows(db, aliceNewRow1, insertStmt)

	aliceNewRow2 := nodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 22)
	insertRows(db, aliceNewRow2, insertStmt)

	aliceNewRow3 := nodeReputationRecordMorphism(aliceNewRep, uptimeColumn, overWrite, 30)
	insertRows(db, aliceNewRow3, insertStmt)

	newAndOldRows, _ := getNodeReputationRecords(db, selectAllStmt)
	if len(newAndOldRows) != 9 {
		t.Error(
			"expected 9 rows in the db got", newAndOldRows,
		)
	}

	morphRep, _ := endianReputation(db, selectAllStmt)

	if morphRep.nodeName != "Alice" {
		t.Error(
			"expected Alice got", morphRep.nodeName,
		)
	}

	onlyAlice, _ := getNodeReputationRecords(db, selectAliceStmt)
	if len(onlyAlice) != 5 {
		t.Error(
			"expected 5 rows in the db got", onlyAlice,
		)
	}

	bestAlice, _ := endianReputation(db, selectAliceStmt)
	if bestAlice.uptime != 30 {
		t.Error(
			"expected uptime of 30 got", bestAlice.uptime,
		)
	}
	pruneNodeReputationRecords(db, bestAlice, deletStmt)

	oneAlice, _ := getNodeReputationRecords(db, selectAliceStmt)
	if len(oneAlice) != 1 {
		t.Error(
			"expected 1 row in the db got", oneAlice,
			"the remaining alice should be", bestAlice,
		)
	}

	cleanUpDB(db)

	os.Remove("./TestEndianPrune.db")
}
