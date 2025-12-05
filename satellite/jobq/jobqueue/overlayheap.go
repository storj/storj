// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqueue

import (
	"container/heap"

	"storj.io/storj/satellite/jobq"
)

// overlayHeap is a layer on top of an existing heap which keeps track of a set
// of changes without changing the original heap.
//
// Among other things, this can be used for popping the next N items from the
// heap without altering or copying the heap. This can offer large memory
// savings when the priority queue is large and N is relatively small. If N is
// more than around 50% of len(heapArray), this is probably not the most
// memory-efficient option anymore.
//
// The underlying heap should not be modified outside of the overlayHeap while
// the overlayHeap is active.
type overlayHeap struct {
	// heapArray is a reference to an existing heap array. This slice and this
	// array are never modified through this overlayHeap.
	heapArray []jobq.RepairJob
	// overlay is a map of indexes to indexes in heapArray. The meaning is, if
	// the item at index Key is requested, supply heapArray[Value] instead. This
	// way we can keep track of swaps and pops without disturbing the real heap.
	overlay map[int]int
	// lessFunc is the Less function used by the heap associated with heapArray.
	lessFunc func(i, j int) bool
	// overlayLen keeps track of the length of the 'virtual' heap.
	overlayLen int
}

func newOverlayHeap(heapArray []jobq.RepairJob, lessFunc func(i, j int) bool) *overlayHeap {
	return &overlayHeap{
		heapArray:  heapArray,
		overlay:    make(map[int]int),
		overlayLen: len(heapArray),
		lessFunc:   lessFunc,
	}
}

func (oh *overlayHeap) getIndex(i int) int {
	if i >= oh.overlayLen {
		panic("overlayHeap index out of bounds")
	}
	if overrideIndex, ok := oh.overlay[i]; ok {
		return overrideIndex
	}
	return i
}

func (oh *overlayHeap) Len() int {
	return oh.overlayLen
}

func (oh *overlayHeap) Less(i, j int) bool {
	return oh.lessFunc(oh.getIndex(i), oh.getIndex(j))
}

func (oh *overlayHeap) Swap(i, j int) {
	oh.overlay[i], oh.overlay[j] = oh.getIndex(j), oh.getIndex(i)
}

func (oh *overlayHeap) Pop() any {
	item := oh.heapArray[oh.getIndex(oh.overlayLen-1)]
	oh.overlayLen--
	delete(oh.overlay, oh.overlayLen)
	return item
}

func (oh *overlayHeap) Push(x any) {
	// overlayHeap is not structured to allow pushing.
	panic("overlayHeap does not support Push")
}

// Peek returns the top element of the heap without removing it.
//
// This is not part of the heap.Interface, but is useful when combining results
// from several overlay heaps.
func (oh *overlayHeap) Peek() jobq.RepairJob {
	return oh.heapArray[oh.getIndex(0)]
}

var _ heap.Interface = (*overlayHeap)(nil)
