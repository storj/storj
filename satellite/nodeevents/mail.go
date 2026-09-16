// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

// NodeSummary identifies a single node listed in a node event email.
type NodeSummary struct {
	ID      string
	Address string
}

// maxNodesPerEmail bounds how many nodes are listed in one email. A batch
// covers every unsent event for an operator, which for a large fleet would
// otherwise render a message big enough for the mail relay to reject, leaving
// those operators silently un-notified.
const maxNodesPerEmail = 100

// NodeEventEmail is the mail service message for a batch of node events which
// share an operator email address and event type.
type NodeEventEmail struct {
	Event     Type
	Satellite string
	Nodes     []NodeSummary
	// More is the number of affected nodes not listed in Nodes.
	More int
}

type nodeEventEmail struct {
	template string
	subject  string
}

var nodeEventEmails = map[Type]nodeEventEmail{
	Online:                  {template: "NodeOnline", subject: "Storage node back online"},
	Offline:                 {template: "NodeOffline", subject: "Storage node offline"},
	Disqualified:            {template: "NodeDisqualified", subject: "Storage node disqualified"},
	UnknownAuditSuspended:   {template: "NodeSuspended", subject: "Storage node suspended"},
	UnknownAuditUnsuspended: {template: "NodeUnsuspended", subject: "Storage node no longer suspended"},
	OfflineSuspended:        {template: "NodeOfflineSuspended", subject: "Storage node suspended for downtime"},
	OfflineUnsuspended:      {template: "NodeOfflineUnsuspended", subject: "Storage node no longer suspended for downtime"},
	BelowMinVersion:         {template: "NodeBelowMinVersion", subject: "Storage node software update required"},
}

// Template returns the email template name.
func (e *NodeEventEmail) Template() string {
	return nodeEventEmails[e.Event].template
}

// Subject returns the email subject.
func (e *NodeEventEmail) Subject() string {
	return nodeEventEmails[e.Event].subject
}
