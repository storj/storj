// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import "github.com/zeebo/errs"

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
