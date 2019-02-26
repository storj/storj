// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package consoleql

const (
	// ActivationPath is key for pass which handles account activation
	ActivationPath = "activationPath"

	// ActivationSubject activation email subject
	ActivationSubject = "Activate your email"
	// InvitationSubject invitation email subject
	InvitationSubject = ""
	// ForgotPasswordSubject forgot password email subject
	ForgotPasswordSubject = ""
)

// MailTemplate is implementation of satellite/mailservice.Template interface
type MailTemplate struct {
	subject  string
	template string
}

// NewMailTemplate creates new instance of MailTemplate
func NewMailTemplate(subject, prefix string) MailTemplate {
	return MailTemplate{
		subject:  subject,
		template: prefix,
	}
}

// Template returns email template name
func (tmpl *MailTemplate) Template() string {
	return tmpl.template
}

// Subject gets email subject
func (tmpl *MailTemplate) Subject() string {
	return tmpl.subject
}

// AccountActivationEmail is mailservice template with activation data
type AccountActivationEmail struct {
	MailTemplate
	ActivationLink string
}

// ForgotPasswordEmail is mailservice template with reset password data
type ForgotPasswordEmail struct {
	MailTemplate
	UserName  string
	ResetLink string
}

// ProjectInvitationEmail is mailservice template for project invitation email
type ProjectInvitationEmail struct {
	MailTemplate
	UserName    string
	ProjectName string
}
