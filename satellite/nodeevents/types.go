// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

// Type is a type of node event.
type Type int

const (
	// Online indicates that the node has come back online.
	Online Type = iota
	// Offline indicates that the node is offline.
	Offline
	// Disqualified indicates that the node is disqualified.
	Disqualified
	// UnknownAuditSuspended indicates that the node is suspended for unknown audit errors.
	UnknownAuditSuspended
	// UnknownAuditUnsuspended indicates that the node is no longer suspended for unknown audit errors.
	UnknownAuditUnsuspended
	// OfflineSuspended indicates that the node is suspended for being offline.
	OfflineSuspended
	// OfflineUnsuspended indicates that the node is no longer suspended for being offline.
	OfflineUnsuspended
	// BelowMinVersion indicates that the node's software is below the minimum version.
	BelowMinVersion
)
