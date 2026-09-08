// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/shared/location"
)

var errTagsNotFound = errs.New("tags not found")

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
	return NodeTag{}, errTagsNotFound
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
	// free disk space in bytes
	FreeDisk int64
}

// Clone returns a deep clone of the selected node.
func (node *SelectedNode) Clone() *SelectedNode {
	newNode := *node
	newNode.Address = pb.CopyNodeAddress(node.Address)
	newNode.Tags = slices.Clone(node.Tags)
	return &newNode
}

// NodeAttribute returns a string (like last_net or tag value) for each SelectedNode.
// can be used to group / label nodes.
type NodeAttribute func(SelectedNode) string

// NodeAttributeInit resolves a NodeAttribute for a specific set of nodes. Most of the attributes
// are simple per node functions (see StaticAttribute), but some of them (like GroupAttribute) can
// be calculated only by knowing all the nodes.
type NodeAttributeInit func(nodes []*SelectedNode) NodeAttribute

// StaticAttribute turns a simple per node attribute to a NodeAttributeInit.
func StaticAttribute(attribute NodeAttribute) NodeAttributeInit {
	return func([]*SelectedNode) NodeAttribute {
		return attribute
	}
}

type unpartitionedNodesKey struct{}

// withUnpartitionedNodes remembers the node set which was handed to the selector chain, before any
// selector in it could narrow the set. It is called once, at the root of the chain (see
// SelectorFromString), so nothing below it has to know about the mechanism.
func withUnpartitionedNodes(ctx context.Context, nodes []*SelectedNode) context.Context {
	return context.WithValue(ctx, unpartitionedNodesKey{}, nodes)
}

// unpartitionedNodes returns the node set before any selector split it up, or nodes, if the
// selector chain wasn't built by SelectorFromString (programmatic selectors don't narrow the set
// under a set aware attribute).
//
// Set aware attributes (see NodeAttributeInit) must be resolved on this set: selectors like
// UnvettedSelector or FilteredSelector init their delegate once per partition, and a group can be
// connected by a node which is in the other partition (or filtered out altogether).
func unpartitionedNodes(ctx context.Context, nodes []*SelectedNode) []*SelectedNode {
	if all, ok := ctx.Value(unpartitionedNodesKey{}).([]*SelectedNode); ok {
		return all
	}
	return nodes
}

// NodeAttributes is a collection of multiple NodeAttribute.
func NodeAttributes(attributes []NodeAttribute, separator string) func(node SelectedNode) string {
	return func(node SelectedNode) string {
		var result []string
		for _, attr := range attributes {
			val := attr(node)
			if val != "" {
				result = append(result, val)
			}
		}
		return strings.Join(result, separator)
	}
}

// NodeValue returns a numerical value for each node.
type NodeValue func(node SelectedNode) float64

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

// CreateNodeValue creates a NodeValue from a string definition.
func CreateNodeValue(attr string) (NodeValue, error) {
	if after, ok := strings.CutPrefix(attr, "tag:"); ok {
		signer, tagName, ok := strings.Cut(strings.TrimSpace(after), "/")
		if !ok {
			return nil, errs.New("tag attribute should be defined as`tag:signer/key or tag:signer/key?default`")
		}

		id, err := storj.NodeIDFromString(signer)
		if err != nil {
			return nil, errs.New("node attribute definition (%s) has invalid NodeID: %s", attr, err.Error())
		}
		var defaultValue float64
		name, defaultVal, withDefault := strings.Cut(tagName, "?")
		if withDefault {
			val, err := strconv.ParseFloat(defaultVal, 64)
			if err != nil {
				return nil, errs.New("node attribute definition (%s) has invalid default value (must be float): %s", attr, err.Error())
			}
			defaultValue = val
		}
		return func(node SelectedNode) float64 {
			tag, err := node.Tags.FindBySignerAndName(id, name)
			if err != nil {
				return defaultValue
			}
			num, err := strconv.ParseFloat(string(tag.Value), 64)
			if err != nil {
				return defaultValue
			}
			return num
		}, nil
	}
	switch attr {
	case "free_disk":
		return func(node SelectedNode) float64 {
			return float64(node.FreeDisk)
		}, nil
	case "piece_count":
		return func(node SelectedNode) float64 {
			return float64(node.PieceCount)
		}, nil
	default:
		return nil, errors.New("Unsupported node value: " + attr)
	}
}

// CreateNodeAttribute creates the NodeAttribute selected based on a string definition.
func CreateNodeAttribute(attr string) (NodeAttribute, error) {
	if after, ok := strings.CutPrefix(attr, "tag:"); ok {
		parts := strings.Split(strings.TrimSpace(after), "/")
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
	case "id", "node_id":
		return func(node SelectedNode) string {
			return node.ID.String()
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
			return strconv.FormatBool(node.Vetted)
		}, nil
	default:
		return nil, errors.New("Unsupported node attribute: " + attr)
	}
}
