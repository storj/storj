// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

// ProjectUsageThreshold identifies a storage or egress usage threshold for a project.
// Its integer value corresponds to a bit in project.notification_flags, so it serves
// double duty as both the event kind and the "email already sent" flag.
type ProjectUsageThreshold int

const (
	// StorageNotificationsEnabled indicates that storage limit notification emails are enabled for the project.
	StorageNotificationsEnabled ProjectUsageThreshold = 1 << 0 // 0x01
	// StorageUsage80 indicates that storage usage has reached 80% of the custom limit.
	StorageUsage80 ProjectUsageThreshold = 1 << 1 // 0x02
	// StorageUsage100 indicates that storage usage has reached 100% of the custom limit.
	StorageUsage100 ProjectUsageThreshold = 1 << 2 // 0x04
	// EgressNotificationsEnabled indicates that egress limit notification emails are enabled for the project.
	EgressNotificationsEnabled ProjectUsageThreshold = 1 << 3 // 0x08
	// EgressUsage80 indicates that egress usage has reached 80% of the custom limit.
	EgressUsage80 ProjectUsageThreshold = 1 << 4 // 0x10
	// EgressUsage100 indicates that egress usage has reached 100% of the custom limit.
	EgressUsage100 ProjectUsageThreshold = 1 << 5 // 0x20
)
