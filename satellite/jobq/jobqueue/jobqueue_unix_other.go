// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unix && !linux

package jobqueue

import "storj.io/storj/satellite/jobq"

func memRealloc(mem []byte, heap []jobq.RepairJob, newLenRecords int) ([]byte, []jobq.RepairJob, error) {
	newMem, newHeap, err := memAlloc(newLenRecords)
	if err != nil {
		return nil, nil, err
	}
	newHeap = newHeap[:len(heap)]
	copy(newHeap, heap)
	err = memFree(mem)
	if err != nil {
		return nil, nil, err
	}
	return newMem, newHeap, nil
}
