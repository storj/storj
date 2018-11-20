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
	"storj.io/storj/pkg/utils"
)

// IdentityLength is the number of bytes required to represent a node id
const IdentityLength = 32

var (
	ErrNotImplemented = errs.Class("not implemented error")
	// ErrNodeID is used when something goes wrong with a node id
	ErrNodeID   = errs.Class("node ID error")
	// EmptyNodeID is the zero-value for a NodeID
	EmptyNodeID = NodeID([IdentityLength]byte{})
)

// NodeID is a unique node identifier
type NodeID [IdentityLength]byte
type NodeIDList []NodeID

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
		return NodeID{}, ErrNodeID.New("not enough bytes to make a node id; have %d, need %d", bLen, IdentityLength)
	}

	var id NodeID
	copy(id[:], b[:IdentityLength])
	return NodeID(id), nil
}

func NodeIDFromKey(k crypto.PublicKey) (NodeID, error) {
	kb, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return NodeID{}, ErrNodeID.Wrap(err)
	}
	hash := make([]byte, IdentityLength)
	sha3.ShakeSum256(hash, kb)
	return NodeIDFromBytes(hash)
}

// String returns NodeID as hex encoded string
func (id NodeID) String() string {
	return base64.URLEncoding.EncodeToString(id[:])
}

// Bytes returns raw bytes of the id
func (id NodeID) Bytes() []byte { return id[:] }

func (id NodeID) Difficulty() uint16 {
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
	if n[i] == EmptyNodeID || n[j] == EmptyNodeID {
		return n[i] == EmptyNodeID
	}

	bytesI := n[i].Bytes()
	bytesJ := n[j].Bytes()
	for k, v := range bytesI {
		if v != bytesJ[k] {
			return v < bytesJ[k]
		}
		// compare next index
	}
	// identical nodeIDs
	return false
}

func (id NodeID) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

func (id *NodeID) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, id.Bytes())
	return n, nil
}

func (id *NodeID) Unmarshal(data []byte) error {
	var err error
	*id, err = NodeIDFromBytes(data)
	return err
}

func (id *NodeID) Size() int {
	return len(id)
}

// TODO(bryanchriswhite): what should this look like?
func (id NodeID) MarshalJSON() ([]byte, error) {
	return nil, ErrNotImplemented.New("MarshalJSON")
}

// TODO(bryanchriswhite): what should this look like?
func (id *NodeID) UnmarshalJSON(data []byte) error {
	return ErrNotImplemented.New("MarshalJSON")
}

// // only required if the compare option is set
// func (id NodeID) Compare(other NodeID) int {}
// // only required if the equal option is set
// func (id NodeID) Equal(other NodeID) bool {}
// // only required if populate option is set
// func NewPopulatedNodeID(r randyNodeIDhetest) *NodeID {}
