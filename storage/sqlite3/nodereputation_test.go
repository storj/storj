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

func TestMatch(t *testing.T) {
	testDatabaseName := "./TestMatch.db"
	db, _ := startDB(testDatabaseName)

	createReputationTable(db)

	createNewNodeRecord(db, "Alice")
	createNewNodeRecord(db, "Bob")
	createNewNodeRecord(db, "Carol")
	createNewNodeRecord(db, "Dave")
	createNewNodeRecord(db, "Eve")

	err := updateNodeRecord(db, "Alice", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Alice", proto.Feature_LATENCY, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Bob", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Bob", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Bob", proto.Feature_LATENCY, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Carol", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Carol", proto.Feature_LATENCY, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Carol", proto.Feature_LATENCY, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Dave", proto.Feature_UPTIME, proto.UpdateRepValue_POINT_TWENTY_FIVE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Dave", proto.Feature_LATENCY, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Eve", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Eve", proto.Feature_LATENCY, proto.UpdateRepValue_NEGITIVE_POINT_FIVE)
	if err != nil {
		panic(err)
	}

	features := []proto.Feature{proto.Feature_UPTIME, proto.Feature_LATENCY}
	var exclude []string

	nodes, err := matchRepOrder(db, features, exclude)
	if err != nil {
		panic(err)
	}
	fmt.Println(nodes)

	// os.Remove(testDatabaseName)
}
