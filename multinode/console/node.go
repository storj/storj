// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// TODO: should this file be placed outside of console in nodes package?

// Node is a representation of storeganode, that SNO could add to the Multinode Dashboard.
type Node struct {
	ID storj.NodeID
	// OwnderID -  we need this field only if there will be multiple node owners.
	OwnderID uuid.UUID
	// APISecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api. is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
	APISecret []byte
	Tag       string // TODO: create enum or type in future.
}
