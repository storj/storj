// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import "github.com/zeebo/errs"

// AllocationManager manages allocations for file retrieval
type AllocationManager struct {
	TotalAllocated, Used int64
}

// NewAllocationManager returns a new AllocationManager
func NewAllocationManager() *AllocationManager {
	return &AllocationManager{}
}

// AllocationError is a type of error for failures in AllocationManager
var AllocationError = errs.Class("allocation error")

// NextReadSize returns the size to be read from file based on max that can be Used
func (am *AllocationManager) NextReadSize() int64 {
	if am.TotalAllocated <= 0 {
		return 0
	}

	remaining := am.TotalAllocated - am.Used

	if remaining > 32*1024 {
		return 32 * 1024
	}

	return remaining
}

// NewTotal adds another allcoation to the AllocationManager
func (am *AllocationManager) NewTotal(total int64) {
	if total > am.TotalAllocated {
		am.TotalAllocated = total
	}
}

// UseAllocation indicates to the AllocationManager that an Amount was successfully Used for an allocation
func (am *AllocationManager) UseAllocation(amount int64) error {
	if amount > am.TotalAllocated-am.Used {
		return AllocationError.New("can't use unallocated bandwidth")
	}

	am.Used += amount

	return nil
}
