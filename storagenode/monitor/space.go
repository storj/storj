// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import "context"

// SpaceReport is an interface for reporting disk usage.
type SpaceReport interface {

	// PreFlightCheck checks if the disk is ready to use.
	PreFlightCheck(ctx context.Context) error

	// AvailableSpace returns available disk space.
	// Used for reporting it to satellite + checking it before upload.
	AvailableSpace(ctx context.Context) (_ int64, err error)

	// DiskSpace returns consolidated disk space state info.
	// Used by reporting only.
	DiskSpace(ctx context.Context) (_ DiskSpace, err error)
}
