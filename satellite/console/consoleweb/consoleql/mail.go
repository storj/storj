// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package consoleql

const (
	// ActivationPath is key for path which handles account activation
	ActivationPath = "activationPath"
	// SignInPath is key for sign in server route
	SignInPath = "signInPath"
)

// AccountActivationEmail is mailservice template with activation data
type AccountActivationEmail struct {
	Origin         string
	ActivationLink string
}

// Template returns email template name
func (*AccountActivationEmail) Template() string { return "Welcome" }

// Subject gets email subject
func (*AccountActivationEmail) Subject() string { return "Activate your email" }

// ForgotPasswordEmail is mailservice template with reset password data
type ForgotPasswordEmail struct {
	Origin        string
	UserName      string
	ResetLink     string
	LetUsKnowLink string
}

// Template returns email template name
func (*ForgotPasswordEmail) Template() string { return "Forgot" }

// Subject gets email subject
func (*ForgotPasswordEmail) Subject() string { return "" }

// ProjectInvitationEmail is mailservice template for project invitation email
type ProjectInvitationEmail struct {
	Origin      string
	UserName    string
	ProjectName string
	SignInLink  string
}

// Template returns email template name
func (*ProjectInvitationEmail) Template() string { return "Invite" }

// Subject gets email subject
func (email *ProjectInvitationEmail) Subject() string {
	return "You were invited to join the Project " + email.ProjectName
}
