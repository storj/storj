// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import "github.com/zeebo/errs"

// Type is a type of node event.
type Type int

const (
	// Online indicates that the node has come back online.
	Online Type = 0
	// Offline indicates that the node is offline.
	Offline Type = 1
	// Disqualified indicates that the node is disqualified.
	Disqualified Type = 2
	// UnknownAuditSuspended indicates that the node is suspended for unknown audit errors.
	UnknownAuditSuspended Type = 3
	// UnknownAuditUnsuspended indicates that the node is no longer suspended for unknown audit errors.
	UnknownAuditUnsuspended Type = 4
	// OfflineSuspended indicates that the node is suspended for being offline.
	OfflineSuspended Type = 5
	// OfflineUnsuspended indicates that the node is no longer suspended for being offline.
	OfflineUnsuspended Type = 6
	// BelowMinVersion indicates that the node's software is below the minimum version.
	BelowMinVersion Type = 7

	onlineName                  = "online"
	offlineName                 = "offline"
	disqualifiedName            = "disqualified"
	unknownAuditSuspendedName   = "unknown audit suspended"
	unknownAuditUnsuspendedName = "unknown audit unsuspended"
	offlineSuspendedName        = "offline suspended"
	offlineUnsuspendedName      = "offline unsuspended"
	belowMinVersionName         = "below minimum version"
)

// Name returns the name of the node event Type.
func (t Type) Name() (name string, err error) {
	switch t {
	case Online:
		name = onlineName
	case Offline:
		name = offlineName
	case Disqualified:
		name = disqualifiedName
	case UnknownAuditSuspended:
		name = unknownAuditSuspendedName
	case UnknownAuditUnsuspended:
		name = unknownAuditUnsuspendedName
	case OfflineSuspended:
		name = offlineSuspendedName
	case OfflineUnsuspended:
		name = offlineUnsuspendedName
	case BelowMinVersion:
		name = belowMinVersionName
	default:
		err = errs.New("invalid Type")
	}
	return name, err
}
