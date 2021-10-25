// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package uploadselection

import "storj.io/common/storj"

// Criteria to filter nodes.
type Criteria struct {
	ExcludeNodeIDs     []storj.NodeID
	AutoExcludeSubnets map[string]struct{} // initialize it with empty map to keep only one node per subnet.
}

// MatchInclude returns with true if node is selected.
func (c *Criteria) MatchInclude(node *Node) bool {
	if ContainsID(c.ExcludeNodeIDs, node.ID) {
		return false
	}
	if c.AutoExcludeSubnets != nil {
		if _, excluded := c.AutoExcludeSubnets[node.LastNet]; excluded {
			return false
		}
		c.AutoExcludeSubnets[node.LastNet] = struct{}{}
	}
	return true
}

// ContainsID returns whether ids contain id.
func ContainsID(ids []storj.NodeID, id storj.NodeID) bool {
	for _, k := range ids {
		if k == id {
			return true
		}
	}
	return false
}
