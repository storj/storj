// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"fmt"
	"os"
	"testing"

	proto "storj.io/storj/protos/nodereputation"
)

func TestCreateTable(t *testing.T) {
	testDatabaseName := "./TestCreateTable.db"
	db, _ := startDB(testDatabaseName)

	createReputationTable(db)

	os.Remove(testDatabaseName)
}

func TestCreateNode(t *testing.T) {
	testDatabaseName := "./TestCreateNode.db"
	db, _ := startDB(testDatabaseName)

	createReputationTable(db)

	createNewNodeRecord(db, "Alice")

	os.Remove(testDatabaseName)
}

func TestSelectReputation(t *testing.T) {
	testDatabaseName := "./TestSelectReputation.db"
	db, _ := startDB(testDatabaseName)

	createReputationTable(db)

	createNewNodeRecord(db, "Alice")

	nodeFeatures, err := getRep(db, "Alice")
	if err != nil {
		panic(err)
	}
	fmt.Println(nodeFeatures)
	os.Remove(testDatabaseName)
}

func TestUpdateReputation(t *testing.T) {
	testDatabaseName := "./TestUpdateReputation.db"
	db, _ := startDB(testDatabaseName)

	createReputationTable(db)

	createNewNodeRecord(db, "Alice")

	nodeFeaturesBefore, err := getRep(db, "Alice")
	if err != nil {
		panic(err)
	}
	fmt.Println(nodeFeaturesBefore)

	err = updateNodeRecord(db, "Alice", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}

	nodeFeaturesAfter, err := getRep(db, "Alice")
	if err != nil {
		panic(err)
	}
	fmt.Println(nodeFeaturesAfter)

	os.Remove(testDatabaseName)
}
