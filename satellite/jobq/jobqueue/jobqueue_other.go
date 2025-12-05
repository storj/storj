// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !unix

package jobqueue

import (
	"storj.io/storj/satellite/jobq"
)

func memAlloc(lenRecords int) ([]byte, []jobq.RepairJob, error) {
	heap := make([]jobq.RepairJob, 0, lenRecords)
	return nil, heap, nil
}

func memRealloc(_ []byte, heap []jobq.RepairJob, newLenRecords int) ([]byte, []jobq.RepairJob, error) {
	newHeap := make([]jobq.RepairJob, 0, newLenRecords)
	newHeap = newHeap[:len(heap)]
	copy(newHeap, heap)
	return nil, newHeap, nil
}

func memFree(_ []byte) error {
	return nil
}

func markUnused(mem []byte, heap []jobq.RepairJob, index int, highWater, _ int) (newHighWater int, err error) {
	// Zero out the bytes of the memory backing array at the given index (job
	// index, not byte index). Since we do this after every Pop, this should
	// result in the array containing all zero bytes after the end of the
	// current slice.
	//
	// This may allow the OS to unmap, efficiently compress, or efficiently
	// merge the corresponding memory pages, allowing memory reclamation
	// after the queue shrinks significantly.
	//
	// At some point, if this needs to run on Windows, it might be worth
	// doing VirtualAlloc(MEM_RELEASE) here, similarly to the unix version.
	//
	// Note that this approach does not work well with Truncate operations,
	// because we don't want to take the time to zero out all records when
	// the queue might be very large.
	heap = heap[:index+1]
	heap[index] = jobq.RepairJob{}
	return highWater, nil
}
