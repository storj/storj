// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package jobqueue

import (
	"unsafe"

	"golang.org/x/sys/unix"

	"storj.io/storj/satellite/jobq"
)

func memRealloc(mem []byte, heap []jobq.RepairJob, newLenRecords int) ([]byte, []jobq.RepairJob, error) {
	newSize := int(jobq.RecordSize) * newLenRecords
	newMem, err := unix.Mremap(mem, newSize, unix.MREMAP_MAYMOVE)
	if err != nil {
		return nil, nil, err
	}
	newHeap := unsafe.Slice((*jobq.RepairJob)(unsafe.Pointer(&newMem[0])), newLenRecords)
	return newMem, newHeap[:len(heap)], nil
}
