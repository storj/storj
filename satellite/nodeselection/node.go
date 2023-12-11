// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"errors"
	"strings"
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

// NodeAttribute returns a string (like last_net or tag value) for each SelectedNode.
type NodeAttribute func(SelectedNode) string

// LastNetAttribute is used for subnet based declumping/selection.
var LastNetAttribute = mustCreateNodeAttribute("last_net")

func mustCreateNodeAttribute(attr string) NodeAttribute {
	nodeAttr, err := CreateNodeAttribute(attr)
	if err != nil {
		panic(err)
	}
	return nodeAttr
}

// CreateNodeAttribute creates the NodeAttribute selected based on a string definition.
func CreateNodeAttribute(attr string) (NodeAttribute, error) {
	if strings.HasPrefix(attr, "tag:") {
		parts := strings.Split(strings.TrimPrefix(attr, "tag:"), "/")
		return func(node SelectedNode) string {
			for _, tag := range node.Tags {
				if tag.Name == parts[1] && tag.Signer.String() == parts[0] {
					return string(tag.Value)
				}
			}
			return ""
		}, nil
	}
	switch attr {
	case "last_net":
		return func(node SelectedNode) string {
			return node.LastNet
		}, nil
	case "wallet":
		return func(node SelectedNode) string {
			return node.Wallet
		}, nil
	case "email":
		return func(node SelectedNode) string {
			return node.Email
		}, nil
	case "country":
		return func(node SelectedNode) string {
			return node.CountryCode.String()
		}, nil
	default:
		return nil, errors.New("Unsupported node attribute: " + attr)
	}
}
