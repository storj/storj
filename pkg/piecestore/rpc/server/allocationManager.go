// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import "github.com/zeebo/errs"

// AllocationManager manages allocations for file retrieval
type AllocationManager struct {
	Allocated, Used, MaxToUse int64
	allocations               []int64
}

// AllocationError is a type of error for failures in AllocationManager
var AllocationError = errs.Class("allocation error")

// NewAllocationManager returns a new AllocationManager
func NewAllocationManager(retrievalSize int64) *AllocationManager {
	return &AllocationManager{MaxToUse: retrievalSize}
}

func (am *AllocationManager) currentAllocation() int64 {
	if (len(am.allocations)) == 0 {
		return 0
	}

	return am.allocations[len(am.allocations)-1]
}

// NextReadSize returns the size to be read from file based on max that can be Used
func (am *AllocationManager) NextReadSize() int64 {
	currentAlloc := am.currentAllocation()

	if currentAlloc > am.MaxToUse-am.Used {
		return am.MaxToUse - am.Used
	}

	return currentAlloc
}

// AddAllocation adds another allcoation to the AllocationManager
func (am *AllocationManager) AddAllocation(allocation int64) {
	if allocation <= 0 {
		return
	}

	am.allocations = append(am.allocations, allocation)
	am.Allocated += allocation
}

// UseAllocation indicates to the AllocationManager that an Amount was successfully Used for an allocation
func (am *AllocationManager) UseAllocation(amount int64) error {
	if (len(am.allocations)) == 0 {
		return AllocationError.New("Used Unallocated Bandwidth")
	}

	am.Used += amount
	am.allocations[len(am.allocations)-1] -= amount

	if am.allocations[len(am.allocations)-1] <= 0 {
		am.allocations = am.allocations[:len(am.allocations)-1]
	}

	return nil
}
