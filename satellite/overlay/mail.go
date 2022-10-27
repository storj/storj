// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package overlay

// NodeOnlineEmail is mailservice template with node online data.
type NodeOnlineEmail struct {
	Origin    string
	NodeID    string
	Satellite string
}

// Template returns email template name.
func (*NodeOnlineEmail) Template() string { return "NodeOnline" }

// Subject gets email subject.
func (*NodeOnlineEmail) Subject() string { return "Your node is back online" }
