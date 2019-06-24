// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"storj.io/storj/storage"
)

type antechamber struct {
	db storage.KeyValueStore
}

func newAntechamber(db storage.KeyValueStore) *antechamber{
	return &antechamber{db: db}
}

func addNode(){}

func removeNode(){}

func updateNeighborhood(){}
