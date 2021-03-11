// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

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
	ID storj.NodeID `json:"id"`
	// APISecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
	APISecret     []byte `json:"apiSecret"`
	PublicAddress string `json:"publicAddress"`
	Name          string `json:"name"`
}

// NodeInfo contains basic node internal state.
type NodeInfo struct {
	ID            storj.NodeID `json:"id"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	LastContact   time.Time    `json:"lastContact"`
	DiskSpaceUsed int64        `json:"diskSpaceUsed"`
	DiskSpaceLeft int64        `json:"diskSpaceLeft"`
	BandwidthUsed int64        `json:"bandwidthUsed"`
	TotalEarned   int64        `json:"totalEarned"`
}

// NodeInfoSatellite contains satellite specific node internal state.
type NodeInfoSatellite struct {
	ID              storj.NodeID `json:"id"`
	Name            string       `json:"name"`
	Version         string       `json:"version"`
	LastContact     time.Time    `json:"lastContact"`
	OnlineScore     float64      `json:"onlineScore"`
	AuditScore      float64      `json:"auditScore"`
	SuspensionScore float64      `json:"suspensionScore"`
	TotalEarned     int64        `json:"totalEarned"`
}
