// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
)

// NodeTag is a tag associated with a node (approved by signer).
type NodeTag struct {
	NodeID   storj.NodeID
	SignedAt time.Time
	Signer   storj.NodeID
	Name     string
	Value    []byte
}

// NodeTags is a collection of multiple NodeTag.
type NodeTags []NodeTag

// FindBySignerAndName selects first tag with same name / NodeID.
func (n NodeTags) FindBySignerAndName(signer storj.NodeID, name string) (NodeTag, error) {
	for _, tag := range n {
		if tag.Name == name && signer == tag.Signer {
			return tag, nil
		}
	}
	return NodeTag{}, errs.New("tags not found")
}

// SelectedNode is used as a result for creating orders limits.
type SelectedNode struct {
	ID          storj.NodeID
	Address     *pb.NodeAddress
	Email       string
	Wallet      string
	LastNet     string
	LastIPPort  string
	CountryCode location.CountryCode
	Exiting     bool
	Suspended   bool
	Online      bool
	Vetted      bool
	Tags        NodeTags
}

// Clone returns a deep clone of the selected node.
func (node *SelectedNode) Clone() *SelectedNode {
	newNode := *node
	newNode.Address = pb.CopyNodeAddress(node.Address)
	newNode.Tags = slices.Clone(node.Tags)
	return &newNode
}
