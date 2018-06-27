// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"os"
	"testing"

	proto "storj.io/storj/protos/nodereputation"
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

	params := []paramValue{
		paramValue{
			param: proto.Parameter_BAD_RECALL,
			val:   0.995,
		},
		paramValue{
			param: proto.Parameter_GOOD_RECALL,
			val:   0.995,
		},
		paramValue{
			param: proto.Parameter_WEIGHT_DENOMINATOR,
			val:   10000.0,
		},
	}
	createNewNodeRecord(db, "Alice", params)

	// os.Remove(testTableName)
}
