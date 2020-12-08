// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"encoding/base64"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// TODO: should this file be placed outside of console in nodes package?

// Nodes exposes needed by MND NodesDB functionality.
//
// architecture: Database
type Nodes interface {
	// GetByID return node from NodesDB by its id.
	GetByID(ctx context.Context, id storj.NodeID) (Node, error)
	// GetAll returns all connected nodes.
	GetAll(ctx context.Context) ([]Node, error)
	// Add creates new node in NodesDB.
	Add(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) error
	// Remove removed node from NodesDB.
	Remove(ctx context.Context, id storj.NodeID) error
}

// ErrNoNode is a special error type that indicates about absence of node in NodesDB.
var ErrNoNode = errs.Class("no such node")

// Node is a representation of storeganode, that SNO could add to the Multinode Dashboard.
type Node struct {
	ID storj.NodeID
	// APISecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api. is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
	APISecret     []byte
	PublicAddress string
	Name          string
}

// APISecretFromBase64 decodes API secret from base 64 string.
func APISecretFromBase64(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}
