// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package console

import (
	"net/mail"
)

// MailTemplate is implementation of satellite/mail.Template interface
type MailTemplate struct {
	to            mail.Address
	subject       string
	htmlPath      string
	plainTextPath string
}

// NewMailTemplate creates new instance of MailTemplate
func NewMailTemplate(to mail.Address, subject, hpath, tpath string) MailTemplate {
	return MailTemplate{
		to:            to,
		subject:       subject,
		htmlPath:      hpath,
		plainTextPath: tpath,
	}
}

// To gets recipients mail addresses
func (tmpl *MailTemplate) To() []mail.Address {
	return []mail.Address{tmpl.to}
}

// Subject gets email subject
func (tmpl *MailTemplate) Subject() string {
	return tmpl.subject
}

// HTMLPath gets path to html template
func (tmpl *MailTemplate) HTMLPath() string {
	return tmpl.htmlPath
}

// PainTextPath gets path to text template
func (tmpl *MailTemplate) PainTextPath() string {
	return tmpl.plainTextPath
}

// AccountActivationEmail is mail template with activation data
type AccountActivationEmail struct {
	MailTemplate
	ActivationLink string
}

// ForgotPasswordEmail is mail template with reset password data
type ForgotPasswordEmail struct {
	MailTemplate
	UserName  string
	ResetLink string
}

// ProjectInvitationEmail is mail template for project invitation email
type ProjectInvitationEmail struct {
	MailTemplate
	UserName    string
	ProjectName string
}
