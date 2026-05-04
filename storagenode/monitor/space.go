// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import "context"

// SpaceReport is an interface for reporting disk usage.
type SpaceReport interface {
	// DiskSpace returns consolidated disk space state info.
	// Used by reporting only.
	DiskSpace(ctx context.Context) (_ DiskSpace, err error)
}
