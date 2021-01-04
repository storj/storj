// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"context"
	"encoding/base64"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// TODO: should this file be placed outside of console in nodes package?

// DB exposes needed by MND NodesDB functionality.
//
// architecture: Database
type DB interface {
	// Get return node from NodesDB by its id.
	Get(ctx context.Context, id storj.NodeID) (Node, error)
	// List returns all connected nodes.
	List(ctx context.Context) ([]Node, error)
	// Add creates new node in NodesDB.
	Add(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) error
	// Remove removed node from NodesDB.
	Remove(ctx context.Context, id storj.NodeID) error
	// UpdateName will update name of the specified node in database.
	UpdateName(ctx context.Context, id storj.NodeID, name string) error
}

// ErrNoNode is a special error type that indicates about absence of node in NodesDB.
var ErrNoNode = errs.Class("no such node")

// Node is a representation of storagenode, that SNO could add to the Multinode Dashboard.
type Node struct {
	ID storj.NodeID
	// APISecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
	APISecret     []byte
	PublicAddress string
	Name          string
}

// APISecretFromBase64 decodes API secret from base 64 string.
func APISecretFromBase64(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}
