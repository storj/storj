// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"os"
	"testing"
)

func TestCreateTable(t *testing.T) {
	db, _ := startDB("./TestCreateTable.db")

	createTable(createStmt, db)

	os.Remove("./TestCreateTable.db")
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
	if op4.naiveScore() != float32(0) {
		t.Error(
			"expected reputation of 0 got", op4.naiveScore(),
		)
	}

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
	aliceNewRep, _ := endianReputation(db, selectAliceStmt)
	aliceNewRow := []nodeReputationRecord{aliceNewRep.morphism(uptimeColumn, overWrite, 20)}

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

	closeDB(db)

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
	aliceNewRep, _ := endianReputation(db, selectAliceStmt)

	insertRows(db,
		[]nodeReputationRecord{aliceNewRep.morphism(uptimeColumn, overWrite, 10)},
		insertStmt,
	)
	insertRows(db,
		[]nodeReputationRecord{aliceNewRep.morphism(uptimeColumn, overWrite, 2)},
		insertStmt,
	)
	insertRows(db,
		[]nodeReputationRecord{aliceNewRep.morphism(uptimeColumn, overWrite, 22)},
		insertStmt,
	)
	insertRows(db,
		[]nodeReputationRecord{aliceNewRep.morphism(uptimeColumn, overWrite, 30)},
		insertStmt,
	)

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

	closeDB(db)

	os.Remove("./TestEndianPrune.db")
}
