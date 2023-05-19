// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package console

import "time"

// AccountActivationEmail is mailservice template with activation data.
type AccountActivationEmail struct {
	Origin                string
	ActivationLink        string
	ContactInfoURL        string
	TermsAndConditionsURL string
}

// Template returns email template name.
func (*AccountActivationEmail) Template() string { return "Welcome" }

// Subject gets email subject.
func (*AccountActivationEmail) Subject() string { return "Activate your email" }

// ForgotPasswordEmail is mailservice template with reset password data.
type ForgotPasswordEmail struct {
	Origin                     string
	UserName                   string
	ResetLink                  string
	CancelPasswordRecoveryLink string
	LetUsKnowURL               string
	ContactInfoURL             string
	TermsAndConditionsURL      string
}

// Template returns email template name.
func (*ForgotPasswordEmail) Template() string { return "Forgot" }

// Subject gets email subject.
func (*ForgotPasswordEmail) Subject() string { return "Password recovery request" }

// ProjectInvitationEmail is mailservice template for project invitation email.
type ProjectInvitationEmail struct {
	Origin                string
	UserName              string
	InviterEmail          string
	SignInLink            string
	LetUsKnowURL          string
	ContactInfoURL        string
	TermsAndConditionsURL string
}

// Template returns email template name.
func (*ProjectInvitationEmail) Template() string { return "Invite" }

// Subject gets email subject.
func (email *ProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// ExistingUserProjectInvitationEmail is mailservice template for project invitation email for existing users.
type ExistingUserProjectInvitationEmail struct {
	InviterEmail string
	Region       string
	SignInLink   string
}

// Template returns email template name.
func (*ExistingUserProjectInvitationEmail) Template() string { return "ExistingUserInvite" }

// Subject gets email subject.
func (email *ExistingUserProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// NewUserProjectInvitationEmail is mailservice template for project invitation email for new users.
type NewUserProjectInvitationEmail struct {
	InviterEmail string
	Region       string
	SignUpLink   string
}

// Template returns email template name.
func (*NewUserProjectInvitationEmail) Template() string { return "NewUserInvite" }

// Subject gets email subject.
func (email *NewUserProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// UnknownResetPasswordEmail is mailservice template with unknown password reset data.
type UnknownResetPasswordEmail struct {
	Satellite           string
	Email               string
	DoubleCheckLink     string
	ResetPasswordLink   string
	CreateAnAccountLink string
	SupportTeamLink     string
}

// Template returns email template name.
func (*UnknownResetPasswordEmail) Template() string { return "UnknownReset" }

// Subject gets email subject.
func (*UnknownResetPasswordEmail) Subject() string {
	return "You have requested to reset your password, but..."
}

// AccountAlreadyExistsEmail is mailservice template for email where user tries to create account, but one already exists.
type AccountAlreadyExistsEmail struct {
	Origin            string
	SatelliteName     string
	SignInLink        string
	ResetPasswordLink string
	CreateAccountLink string
}

// Template returns email template name.
func (*AccountAlreadyExistsEmail) Template() string { return "AccountAlreadyExists" }

// Subject gets email subject.
func (*AccountAlreadyExistsEmail) Subject() string {
	return "Are you trying to sign in?"
}

// LockAccountEmail is mailservice template with lock account data.
type LockAccountEmail struct {
	Name              string
	LockoutDuration   time.Duration
	ResetPasswordLink string
}

// Template returns email template name.
func (*LockAccountEmail) Template() string { return "LockAccount" }

// Subject gets email subject.
func (*LockAccountEmail) Subject() string { return "Account Lock" }
