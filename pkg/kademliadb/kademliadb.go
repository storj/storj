// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package kademliadb

import (
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	// kademliaDBError is the class for all errors pertaining to kademliaDB operations
	kademliaDBError = errs.Class("kademlia DB error")
	mon        = monkit.Package()
)

type kademliaDB struct {
	KBuckets *KBuckets
	Nodes *Nodes
	Antechamber *Nodes
}

func New() *kademliaDB {
	//TODO
}