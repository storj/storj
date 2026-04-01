// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

// RunEarly is a marker annotation for components that should be initialized and started
// before the INITIALIZATION of other components. Components tagged with RunEarly will:
// 1. Be initialized first (along with their dependencies)
// 2. Have their Run method called before the initialization other components
// This is useful for infrastructure components that must be running before other services start.
type RunEarly struct{}

func (r RunEarly) String() string {
	return "RunEarly"
}
