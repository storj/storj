// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unix

package jobqueue

import (
	"unsafe"

	"golang.org/x/sys/unix"

	"storj.io/storj/satellite/jobq"
)

// memAlloc allocates memory using mmap and returns it as a byte slice (with the
// new heap pointing at the same memory, casted to RepairJob).
//
// We do this rather than using `make([]RepairJob, ...)` because we don't want
// Go to spend time zeroing out the memory. The initial allocation may be large,
// and we count on the OS leaving unused pages of memory unmapped to minimize
// resource consumption.
func memAlloc(lenRecords int) ([]byte, []jobq.RepairJob, error) {
	mem, err := unix.Mmap(-1, 0, int(jobq.RecordSize)*lenRecords, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		return nil, nil, err
	}
	heap := unsafe.Slice((*jobq.RepairJob)(unsafe.Pointer(&mem[0])), lenRecords)
	return mem, heap[:0], nil
}

func memFree(mem []byte) error {
	return unix.Munmap(mem)
}

const elementsLeaveMapped = 1000

var (
	pageSize = uintptr(unix.Getpagesize())
	pageMask = ^(uint64(pageSize) - 1)
)

func pageOf(p uintptr) uintptr {
	return p & uintptr(pageMask)
}

func markUnused(sp []byte, _ []jobq.RepairJob, index int, highWater, memReleaseThreshold int) (newHighWater int, err error) {
	if highWater-index < memReleaseThreshold {
		return highWater, nil
	}

	// we've now popped enough elements that there are now more than
	// `memReleaseThreshold` empty element spaces in the slice that were
	// once filled (so, presumably, that memory is mapped and doesn't need
	// to be). We now mark _most of_ that memory as unused so the OS can
	// reclaim it, if it is so inclined.
	//
	// We don't mark _all_ of it as unused, to avoid the cost of re-faulting
	// pages back in too frequently in case the size of the queue hovers
	// around this point.
	//
	// Rather than having yet another confusing config item, we will
	// somewhat arbitrarily leave the space for N elements mapped, and mark
	// the rest as unused. One exception: if memReleaseThreshold is quite small,
	// we need to make sure elementsLeaveMapped is smaller to avoid needing to
	// call markUnused with every pop.
	leaveMapped := elementsLeaveMapped
	if leaveMapped > memReleaseThreshold/10 {
		leaveMapped = memReleaseThreshold / 10
	}
	fullSlice := sp[0:cap(sp)]
	startAddr := uintptr(unsafe.Pointer(&fullSlice[0]))
	startReleaseAtIndex := uintptr(index+leaveMapped) * jobq.RecordSize
	if startReleaseAtIndex >= uintptr(cap(sp)) {
		// we're near the end of our memory region, and there isn't enough
		// room to leave N elements mapped before the unmapped area. We can't
		// unmap anything here.
		return highWater, nil
	}
	endReleaseAtIndex := uintptr(highWater) * jobq.RecordSize
	memToReleaseBegin := pageOf(startAddr + startReleaseAtIndex)
	memToReleaseEnd := pageOf(startAddr + endReleaseAtIndex)

	// don't release the first page of this memory, in case there is still
	// valid data in it.
	memToReleaseBegin += pageSize

	// don't release the very last page of the backing array, since there may be
	// memory for other things in that same page, beyond the end of the backing
	// array. we can't take the address of fullSlice[cap(sp)], so we calculate
	// where the end will be with pointer arithmetic.
	lastPage := pageOf(startAddr + uintptr(cap(sp)))
	if memToReleaseEnd == lastPage {
		memToReleaseEnd -= pageSize
	}

	if memToReleaseBegin < memToReleaseEnd {
		offset := memToReleaseBegin - startAddr
		sliceToMarkUnused := sp[offset : offset+(memToReleaseEnd-memToReleaseBegin)]
		err = unix.Madvise(sliceToMarkUnused, unix.MADV_FREE)
		if err != nil {
			return highWater, err
		}
		highWater = index + leaveMapped
	}
	return highWater, nil
}
