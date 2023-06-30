// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uploadselection

import (
	"time"

	"github.com/zeebo/errs"

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
	LastNet     string
	LastIPPort  string
	CountryCode location.CountryCode
	Tags        NodeTags
}

// Clone returns a deep clone of the selected node.
func (node *SelectedNode) Clone() *SelectedNode {
	copy := pb.CopyNode(&pb.Node{Id: node.ID, Address: node.Address})
	tags := make([]NodeTag, len(node.Tags))
	for ix, tag := range node.Tags {
		tags[ix] = NodeTag{
			NodeID:   tag.NodeID,
			SignedAt: tag.SignedAt,
			Signer:   tag.Signer,
			Name:     tag.Name,
			Value:    tag.Value,
		}
	}
	return &SelectedNode{
		ID:          copy.Id,
		Address:     copy.Address,
		LastNet:     node.LastNet,
		LastIPPort:  node.LastIPPort,
		CountryCode: node.CountryCode,
		Tags:        tags,
	}
}
