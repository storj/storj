// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package projectlimitevents

// ProjectStorageUsage80Email is sent when a project's storage usage reaches 80% of its limit.
type ProjectStorageUsage80Email struct {
	ProjectName string
	IsFree      bool
	IsNFR       bool
}

// Template returns the email template name.
func (*ProjectStorageUsage80Email) Template() string { return "ProjectStorageUsage80" }

// Subject returns the email subject.
func (e *ProjectStorageUsage80Email) Subject() string {
	if e.IsFree {
		return "Time to upgrade: You've reached 80% of your storage limit"
	}
	return "Important: You've reached 80% of your storage limit"
}

// ProjectStorageUsage100Email is sent when a project's storage usage reaches 100% of its limit.
type ProjectStorageUsage100Email struct {
	ProjectName string
	IsFree      bool
	IsNFR       bool
}

// Template returns the email template name.
func (*ProjectStorageUsage100Email) Template() string { return "ProjectStorageUsage100" }

// Subject returns the email subject.
func (e *ProjectStorageUsage100Email) Subject() string {
	if e.IsFree {
		return "Time to upgrade: You've reached 100% of your storage limit"
	}
	return "Urgent: You've reached 100% of your storage limit"
}

// ProjectEgressUsage80Email is sent when a project's egress usage reaches 80% of its limit.
type ProjectEgressUsage80Email struct {
	ProjectName string
	IsFree      bool
	IsNFR       bool
}

// Template returns the email template name.
func (*ProjectEgressUsage80Email) Template() string { return "ProjectEgressUsage80" }

// Subject returns the email subject.
func (e *ProjectEgressUsage80Email) Subject() string {
	if e.IsFree {
		return "Time to upgrade: You've reached 80% of your download limit"
	}
	return "Important: You've reached 80% of your download limit"
}

// ProjectEgressUsage100Email is sent when a project's egress usage reaches 100% of its limit.
type ProjectEgressUsage100Email struct {
	ProjectName string
	IsFree      bool
	IsNFR       bool
}

// Template returns the email template name.
func (*ProjectEgressUsage100Email) Template() string { return "ProjectEgressUsage100" }

// Subject returns the email subject.
func (e *ProjectEgressUsage100Email) Subject() string {
	if e.IsFree {
		return "Time to upgrade: You've reached 100% of your download limit"
	}
	return "Urgent: You've reached 100% of your download limit"
}
