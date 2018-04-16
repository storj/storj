package reputation

import (
	"fmt"
	"os"
	"testing"
)

func TestCreateTable(t *testing.T) {
	db := startDB("./TestCreateTable.db")

	createStmt := `
	CREATE table foo (id interger not null primary key, name text);
	`
	createTable(createStmt, db)

	os.Remove("./TestCreateTable.db")
}

func TestRowInsertAndQuery(t *testing.T) {
	db := startDB("./TestRowInsertAndQuery.db")

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
	db := startDB("./TestFindNode.db")

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

	rows := []RepRow{
		RepRow{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		RepRow{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		RepRow{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		RepRow{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		RepRow{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep := naiveReputation(db, selectStmt)

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
	db := startDB("./TestUpdateNode.db")

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

	rows := []RepRow{
		RepRow{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		RepRow{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		RepRow{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		RepRow{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		RepRow{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep := naiveReputation(db, selectStmt)

	if bestRep.name != "Carol" {
		t.Error(
			"expected Carol got", bestRep.name,
			"with a reputation of", bestRep.naiveRep(),
		)
	}

	rows1 := []RepRow{
		RepRow{"Alice", "", 6, 10, 5, 5, 100, 0, 0},
		RepRow{"Bob", "", 11, 20, 0, 10, 100, 0, 0},
		RepRow{"Carol", "", 1, 10, 5, 3, 100, 0, 0},
		RepRow{"Dave", "", 16, 10, 0, 5, 500, 0, 0},
		RepRow{"Eve", "", 6, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows1, insertStmt)
	bestRep1 := naiveReputation(db, selectStmt)

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
	db := startDB("./TestFindNodeNoFail.db")

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

	rows := []RepRow{
		RepRow{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		RepRow{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		RepRow{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		RepRow{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		RepRow{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	bestRep := naiveReputation(db, noFailStmt)

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
	alice := NewReputationRow("Alice") //RepRow{"Alice", "", 5, 10, 5, 5, 100, 0, 0}
	alice = alice.morphism(uptimeColumn, overWrite, 5)
	// col need else NaN
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
	db := startDB("./TestMorphismAndInsertOfRow.db")

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

	rows := []RepRow{
		RepRow{"Alice", "", 5, 10, 5, 5, 100, 0, 0},
		RepRow{"Bob", "", 10, 20, 0, 10, 100, 0, 0},
		RepRow{"Carol", "", 50, 10, 5, 3, 100, 0, 0},
		RepRow{"Dave", "", 15, 10, 0, 5, 500, 0, 0},
		RepRow{"Eve", "", 5, 10, 5, 5, 100, 0, 1},
	}

	insertRows(db, rows, insertStmt)

	aliceRep := naiveReputation(db, selectAliceStmt)

	aliceNaiveRep := fmt.Sprintf("%.2f", aliceRep.naiveRep())

	if aliceNaiveRep != "605.67" {
		t.Error(
			"expected Alice got", aliceRep.name,
			"with a reputation of 605.67", aliceNaiveRep,
		)
	}

	aliceNewRep := getRepRows(db, selectAliceStmt)
	aliceNewRow := repRowMorphism(aliceNewRep, shardsModifiedColumn, increment, 2)

	insertRows(db, aliceNewRow, insertStmt)

	aliceRows := getRepRows(db, selectAliceStmt)
	if len(aliceRows) != 2 {
		t.Error(
			"expected 2 rows in the db got", aliceRows,
		)
	}

	morphRep := naiveReputation(db, selectAliceStmt)

	if morphRep.naiveRep() != float64(0) {
		t.Error(
			"expected reputation of 0 for Alice got", morphRep.naiveRep(),
			"expected Alice with shards modified greater that 0", morphRep,
		)
	}

	cleanUpDB(db)

	os.Remove("./TestMorphismAndInsertOfRow.db")
}
