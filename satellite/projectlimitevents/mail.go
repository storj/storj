// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package projectlimitevents

import "storj.io/common/memory"

// ProjectStorageUsage80Email is sent when a project's storage usage reaches 80% of its limit.
type ProjectStorageUsage80Email struct {
	ProjectName string
	Limit       memory.Size
}

// Template returns the email template name.
func (*ProjectStorageUsage80Email) Template() string { return "ProjectStorageUsage80" }

// Subject returns the email subject.
func (*ProjectStorageUsage80Email) Subject() string {
	return "Your project storage is 80% full"
}

// ProjectStorageUsage100Email is sent when a project's storage usage reaches 100% of its limit.
type ProjectStorageUsage100Email struct {
	ProjectName string
	Limit       memory.Size
}

// Template returns the email template name.
func (*ProjectStorageUsage100Email) Template() string { return "ProjectStorageUsage100" }

// Subject returns the email subject.
func (*ProjectStorageUsage100Email) Subject() string {
	return "Your project storage limit has been reached"
}

// ProjectEgressUsage80Email is sent when a project's egress usage reaches 80% of its limit.
type ProjectEgressUsage80Email struct {
	ProjectName string
	Limit       memory.Size
}

// Template returns the email template name.
func (*ProjectEgressUsage80Email) Template() string { return "ProjectEgressUsage80" }

// Subject returns the email subject.
func (*ProjectEgressUsage80Email) Subject() string {
	return "Your project download usage is 80% full"
}

// ProjectEgressUsage100Email is sent when a project's egress usage reaches 100% of its limit.
type ProjectEgressUsage100Email struct {
	ProjectName string
	Limit       memory.Size
}

// Template returns the email template name.
func (*ProjectEgressUsage100Email) Template() string { return "ProjectEgressUsage100" }

// Subject returns the email subject.
func (*ProjectEgressUsage100Email) Subject() string {
	return "Your project download limit has been reached"
}
