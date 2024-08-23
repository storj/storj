// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"errors"
	"fmt"
	"net"
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
	PieceCount  int64
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

// Subnet can return the IP network of the node for any netmask length.
func Subnet(bits int64) NodeAttribute {
	return func(node SelectedNode) string {
		addr, _, _ := strings.Cut(node.LastIPPort, ":")
		_, network, err := net.ParseCIDR(fmt.Sprintf("%s/%d", addr, bits))
		if err != nil {
			return "error:" + err.Error()
		}
		return network.String()
	}
}

func mustCreateNodeAttribute(attr string) NodeAttribute {
	nodeAttr, err := CreateNodeAttribute(attr)
	if err != nil {
		panic(err)
	}
	return nodeAttr
}

// NodeTagAttribute selects a tag value from node.
func NodeTagAttribute(signer storj.NodeID, tagName string) NodeAttribute {
	return func(node SelectedNode) string {
		tag, err := node.Tags.FindBySignerAndName(signer, tagName)
		if err != nil {
			return ""
		}
		return string(tag.Value)
	}
}

// AnyNodeTagAttribute selects a tag value from node, accepts any signer.
func AnyNodeTagAttribute(tagName string) NodeAttribute {
	return func(node SelectedNode) string {
		for _, tag := range node.Tags {
			if tag.Name == tagName {
				return string(tag.Value)
			}
		}
		return ""
	}
}

// CreateNodeAttribute creates the NodeAttribute selected based on a string definition.
func CreateNodeAttribute(attr string) (NodeAttribute, error) {
	if strings.HasPrefix(attr, "tag:") {
		parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(attr, "tag:")), "/")
		switch len(parts) {
		case 1:
			return AnyNodeTagAttribute(parts[0]), nil
		case 2:
			id, err := storj.NodeIDFromString(parts[0])
			if err != nil {
				return nil, errs.New("node attribute definition (%s) has invalid NodeID: %s", attr, err.Error())
			}
			return NodeTagAttribute(id, parts[1]), nil
		default:
			return nil, errs.New("tag attribute should be defined as `tag:key` (any signer) or `tag:signer/key`")
		}
	}
	switch attr {
	case "last_net":
		return func(node SelectedNode) string {
			return node.LastNet
		}, nil
	case "last_ip_port":
		return func(node SelectedNode) string {
			return node.LastIPPort
		}, nil
	case "last_ip":
		return func(node SelectedNode) string {
			ip, _, _ := strings.Cut(node.LastIPPort, ":")
			return ip
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
	case "vetted":
		return func(node SelectedNode) string {
			return fmt.Sprintf("%t", node.Vetted)
		}, nil
	default:
		return nil, errors.New("Unsupported node attribute: " + attr)
	}
}
