// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package platform

// DiskInfo contains information about a disk volume.
type DiskInfo struct {
	// AvailableSpace is the number of bytes available to the current user.
	AvailableSpace uint64
	// DiskID is an opaque identifier for the underlying storage volume. Two
	// paths that share the same DiskID reside on the same disk or volume.
	DiskID string
}
