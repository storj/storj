// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package satellite

const (
	// ActivationPath is key for pass which handles account activation
	ActivationPath = "activationPath"
)

// AccountActivationEmail is mailservice template with activation data
type AccountActivationEmail struct {
	ActivationLink string
}

// Template returns email template name
func (*AccountActivationEmail) Template() string { return "Welcome" }

// Subject gets email subject
func (*AccountActivationEmail) Subject() string { return "Activate your email" }

// ForgotPasswordEmail is mailservice template with reset password data
type ForgotPasswordEmail struct {
	UserName  string
	ResetLink string
}

// Template returns email template name
func (*ForgotPasswordEmail) Template() string { return "Forgot" }

// Subject gets email subject
func (*ForgotPasswordEmail) Subject() string { return "" }

// ProjectInvitationEmail is mailservice template for project invitation email
type ProjectInvitationEmail struct {
	UserName    string
	ProjectName string
}

// Template returns email template name
func (*ProjectInvitationEmail) Template() string { return "Invite" }

// Subject gets email subject
func (*ProjectInvitationEmail) Subject() string { return "" }
