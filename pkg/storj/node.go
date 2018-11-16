// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math/bits"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
)

// IdentityLength is the number of bytes required to represent a node id
const IdentityLength = 32

// ErrNodeID is used when something goes wrong with a node id
var (
	ErrNodeID   = errs.Class("node ID error")
	EmptyNodeID = nodeID([IdentityLength]byte{})
)

type Node struct {
	// NB: `Id` (lowercase "d") is intentional; protobuf's naming convention is incompatible with and trumps go's here
	// 		 `Id` shadows embedded `storj.Node.Id` field
	Id NodeID
	*pb.Node
}

// NodeID is a unique node identifier
type NodeID interface {
	String() string
	Bytes() []byte
	Difficulty() uint16
}

type nodeID [IdentityLength]byte
type NodeIDList []NodeID

func ProtoNodes(n []Node) (pbNodes []*pb.Node) {
	for _, node := range n {
		pbNodes = append(pbNodes, node.Node)
	}
	return pbNodes
}

func NewNode(n *pb.Node) (Node, error) {
	if n == nil {
		return Node{}, nil
	}

	id, err := NodeIDFromBytes(n.GetId())
	if err != nil {
		return Node{}, err
	}

	return Node{ id, n, }, nil
}

func NewNodeWithID(id NodeID, n *pb.Node) Node {
	n.Id = id.Bytes()

	return Node{
		Id: id,
		Node: n,
	}
}

func NewNodes(pbNodes []*pb.Node) ([]Node, error) {
	var (
		nodeErrs []error
		// NB: prevent nil return value
		nodes = []Node{}
	)
	for _, n := range pbNodes {
		node, err := NewNode(n)
		if err != nil {
			nodeErrs = append(nodeErrs, err)
			continue
		}
		nodes = append(nodes, node)
	}
	if err := utils.CombineErrors(nodeErrs...); err != nil {
		return nodes, err
	}

	return nodes, nil
}

func NodeIDFromString(s string) (NodeID, error) {
	idBytes, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return EmptyNodeID, ErrNodeID.Wrap(err)
	}
	return NodeIDFromBytes(idBytes)
}

func NodeIDsFromBytes(b [][]byte) (ids NodeIDList, err error) {
	var idErrs []error
	for _, idBytes := range b {
		id, err := NodeIDFromBytes(idBytes)
		if err != nil {
			idErrs = append(idErrs, err)
			continue
		}

		ids = append(ids, id)
	}

	if err = utils.CombineErrors(idErrs...); err != nil {
		return nil, err
	}
	return ids, nil
}

func NodeIDFromBytes(b []byte) (NodeID, error) {
	bLen := len(b)
	if bLen < IdentityLength {
		return nodeID{}, ErrNodeID.New("not enough bytes to make a node id; have %d, need %d", bLen, IdentityLength)
	}

	var id nodeID
	copy(id[:], b[:IdentityLength])
	return nodeID(id), nil
}

func NodeIDFromKey(k crypto.PublicKey) (NodeID, error) {
	kb, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return nodeID{}, ErrNodeID.Wrap(err)
	}
	hash := make([]byte, IdentityLength)
	sha3.ShakeSum256(hash, kb)
	return NodeIDFromBytes(hash)
}

func NodeIDFromNode(n *pb.Node) (NodeID, error) {
	return NodeIDFromBytes(n.GetId())
}

func (n Node) GetId() NodeID {
	if n.Id == nil {
		return EmptyNodeID
	}
	return n.Id
}

// String returns NodeID as hex encoded string
func (id nodeID) String() string {
	return base64.URLEncoding.EncodeToString(id[:])
}

// Bytes returns raw bytes of the id
func (id nodeID) Bytes() []byte { return id[:] }

func (id nodeID) Difficulty() uint16 {
	idLen := len(id)
	for i := 1; i < idLen; i++ {
		b := id[idLen-i]

		if b != 0 {
			zeroBits := bits.TrailingZeros16(uint16(b))
			if zeroBits == 16 {
				zeroBits = 0
			}

			return uint16((i-1)*8 + zeroBits)
		}
	}

	// NB: this should never happen
	reason := fmt.Sprintf("difficulty matches id hash length: %d; hash (hex): % x", idLen, id)
	zap.S().Error(reason)
	panic(reason)
}

func (n NodeIDList) Bytes() (idsBytes [][]byte) {
	for _, nid := range n {
		idsBytes = append(idsBytes, nid.Bytes())
	}
	return idsBytes
}

func (n NodeIDList) Len() int {
	return len(n)
}

func (n NodeIDList) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n NodeIDList) Less(i, j int) bool {
	if n[i] == nil || n[j] == nil {
		return n[i] == nil
	}

	bytesI := n[i].Bytes()
	bytesJ := n[j].Bytes()
	for k, v := range bytesI {
		if v != bytesJ[k] {
			return  v < bytesJ[k]
		}
		// compare next index
	}
	// identical nodeIDs
	return false
}
