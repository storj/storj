// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"bufio"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"storj.io/common/debug"
	"storj.io/common/storj"
)

type singleNodeStats struct {
	initialSelection atomic.Int64
	retrySelection   atomic.Int64
}

// NodeSelectionStats is a debug.Extension that keeps track of how many times
// a node has been selected.
type NodeSelectionStats struct {
	nodeCounts sync.Map
}

var _ debug.Extension = (*NodeSelectionStats)(nil)

// NewNodeSelectionStats creates a NodeSelectionStats.
func NewNodeSelectionStats() *NodeSelectionStats {
	return &NodeSelectionStats{}
}

// IncrementInitial increments a node's selection count for initial selection.
func (stats *NodeSelectionStats) IncrementInitial(node storj.NodeID) {
	counters, ok := stats.nodeCounts.Load(node)
	if !ok {
		counters, _ = stats.nodeCounts.LoadOrStore(node, &singleNodeStats{})
	}
	counters.(*singleNodeStats).initialSelection.Add(1)
}

// IncrementRetry increments a node's selection count for retry selection.
func (stats *NodeSelectionStats) IncrementRetry(node storj.NodeID) {
	counters, ok := stats.nodeCounts.Load(node)
	if !ok {
		counters, _ = stats.nodeCounts.LoadOrStore(node, &singleNodeStats{})
	}
	counters.(*singleNodeStats).retrySelection.Add(1)
}

// Description implements debug.Extension.
func (stats *NodeSelectionStats) Description() string {
	return "Information about how many times a node has been selected."
}

// Path implements debug.Extension.
func (stats *NodeSelectionStats) Path() string {
	return "/node-selection"
}

// Handler implements debug.Extension.
func (stats *NodeSelectionStats) Handler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/plain")
	buf := bufio.NewWriter(writer)

	_, _ = fmt.Fprintln(buf, "nodeid\tinitial\tretry")

	stats.nodeCounts.Range(func(key, value any) bool {
		node := key.(storj.NodeID)
		counters := value.(*singleNodeStats)
		_, err := fmt.Fprintf(buf, "%s\t%d\t%d\n", node, counters.initialSelection.Load(), counters.retrySelection.Load())
		return err == nil
	})

	_ = buf.Flush()
}
