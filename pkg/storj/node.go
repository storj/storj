// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"math/bits"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

const IDVersion = 0

var (
	// ErrNodeID is used when something goes wrong with a node id
	ErrNodeID = errs.Class("node ID error")
)

// NodeID is a unique node identifier
type NodeID [32]byte
type NodeIDList []NodeID

func NodeIDFromString(s string) (NodeID, error) {
	idBytes, _, err := base58.CheckDecode(s)
	if err != nil {
		return NodeID{}, ErrNodeID.Wrap(err)
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
	if bLen != len(NodeID{}) {
		return NodeID{}, ErrNodeID.New("not enough bytes to make a node id; have %d, need %d", bLen, len(NodeID{}))
	}

	var id NodeID
	copy(id[:], b[:])
	return NodeID(id), nil
}

func (id NodeID) Less(compID NodeID) bool {
	for k, v := range id {
		if v < compID[k] {
			return true
		} else if v > compID[k] {
			return false
		}
		// compare next index
	}
	// identical nodeIDs
	return false
}

// String returns NodeID as hex encoded string
func (id NodeID) String() string {
	return base58.CheckEncode(id[:], IDVersion)
}

// Bytes returns raw bytes of the id
func (id NodeID) Bytes() []byte { return id[:] }

func (id NodeID) Difficulty() (uint16, error) {
	idLen := len(id)
	for i := 1; i < idLen; i++ {
		b := id[idLen-i]

		if b != 0 {
			zeroBits := bits.TrailingZeros16(uint16(b))
			if zeroBits == 16 {
				zeroBits = 0
			}

			return uint16((i-1)*8 + zeroBits), nil
		}
	}

	return 0, ErrNodeID.New("difficulty matches id hash length: %d; hash (hex): % x", idLen, id)
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

func (id NodeID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

func (id *NodeID) UnmarshalJSON(data []byte) error {
	var err error
	*id, err = NodeIDFromString(string(data))
	if err != nil {
		return err
	}
	return nil
}

// // only required if the compare option is set
// func (id NodeID) Compare(other NodeID) int {}
// // only required if the equal option is set
// func (id NodeID) Equal(other NodeID) bool {}
// // only required if populate option is set
// func NewPopulatedNodeID(r randyNodeIDhetest) *NodeID {}

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
	for k, v := range n[i] {
		if v < n[j][k] {
			return true
		} else if v > n[j][k] {
			return false
		}
		// compare next index
	}
	// identical nodeIDs
	return false
}
