// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"os"
	"testing"
)

func TestCreateTable(t *testing.T) {
	testTableName := "./TestCreateTable.db"
	db, _ := startDB(testTableName)

	createReputationTable(db)

	os.Remove(testTableName)
}

func TestCreateNode(t *testing.T) {
	testTableName := "./TestCreateNode.db"
	db, _ := startDB(testTableName)

	createReputationTable(db)

	createNewNodeRecord(db, "Alice")

	os.Remove(testTableName)
}
