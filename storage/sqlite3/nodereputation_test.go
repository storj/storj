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
	StartDB(testDatabaseName)
	os.Remove(testDatabaseName)
}

func TestCreateNode(t *testing.T) {
	testDatabaseName := "./TestCreateNode.db"
	db, _ := StartDB(testDatabaseName)

	CreateNewNodeRecord(db, "Alice")

	os.Remove(testDatabaseName)
}

func TestSelectReputation(t *testing.T) {
	testDatabaseName := "./TestSelectReputation.db"
	db, _ := StartDB(testDatabaseName)

	CreateNewNodeRecord(db, "Alice")

	_, err := GetRep(db, "Alice")
	if err != nil {
		panic(err)
	}

	os.Remove(testDatabaseName)
}

func TestUpdateReputation(t *testing.T) {
	testDatabaseName := "./TestUpdateReputation.db"
	db, _ := StartDB(testDatabaseName)

	CreateNewNodeRecord(db, "Alice")

	_, err := GetRep(db, "Alice")
	if err != nil {
		panic(err)
	}

	err = updateNodeRecord(db, "Alice", proto.Feature_UPTIME, proto.UpdateRepValue_ONE)
	if err != nil {
		panic(err)
	}

	_, err = GetRep(db, "Alice")
	if err != nil {
		panic(err)
	}

	os.Remove(testDatabaseName)
}

func TestMatch(t *testing.T) {
	testDatabaseName := "./TestMatch.db"
	db, _ := StartDB(testDatabaseName)

	CreateNewNodeRecord(db, "Alice")
	CreateNewNodeRecord(db, "Bob")
	CreateNewNodeRecord(db, "Carol")
	CreateNewNodeRecord(db, "Dave")
	CreateNewNodeRecord(db, "Eve")

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
	err = updateNodeRecord(db, "Carol", proto.Feature_UPTIME, proto.UpdateRepValue_POINT_FIVE)
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
	err = updateNodeRecord(db, "Eve", proto.Feature_LATENCY, proto.UpdateRepValue_NEGATIVE_POINT_FIVE)
	if err != nil {
		panic(err)
	}
	err = updateNodeRecord(db, "Eve", proto.Feature_LATENCY, proto.UpdateRepValue_NEGATIVE_POINT_FIVE)
	if err != nil {
		panic(err)
	}

	features := []proto.Feature{proto.Feature_UPTIME, proto.Feature_LATENCY}
	var exclude []string

	nodes, err := matchRepOrder(db, 4, features, exclude)
	if err != nil {
		panic(err)
	}
	fmt.Println(nodes)

	test := []string{"Bob", "Carol", "Alice", "Dave"}

	for i := range test {
		if test[i] != nodes[i] {
			t.Error("expected nodes [Bob Carol Alice Dave] got", nodes)
		}
	}

	os.Remove(testDatabaseName)
}
