// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import (
	"bytes"
	"context"
	"html/template"
	"net/mail"
	"path/filepath"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	m "storj.io/storj/internal/mail"
)

// Config defines values needed by mail service
type Config struct {
	SMTPServerAddress string `help:"smtp server address" default:""`
	TemplatePath      string `help:"path to mail templates source" default:""`
	From              string `help:"sender email address" default:""`
	Auth              struct {
		Type  string `help:"smtp authentication type" default:"OAUTH2"`
		Plain struct {
			Login    string `help:"plain auth user login" default:""`
			Password string `help:"plain auth user password" default:""`
		}
		OAUTH2 struct {
			RefreshToken string `help:"refresh token used to retrieve new access token" default:""`
			Credentials  struct {
				ClientID     string `help:"oauth2 app's client id" default:""`
				ClientSecret string `help:"oauth2 app's client secret" default:""`
				TokenURI     string `help:"uri which is used when retrieving new access token" default:""`
			}
		}
	}
}

var (
	mon = monkit.Package()
)

// Service sends predefined email messages through SMTP
type Service struct {
	log    *zap.Logger
	sender m.SMTPSender

	templatePath string
}

// NewService creates new service
func NewService(log *zap.Logger, sender m.SMTPSender, templatePath string) *Service {
	return &Service{log: log, sender: sender, templatePath: templatePath}
}

// SendActivationEmail sends account activation link email message
func (service *Service) SendActivationEmail(ctx context.Context, to mail.Address, link string) (err error) {
	defer mon.Task()(&ctx)(&err)
	var buffer bytes.Buffer

	template, err := template.ParseFiles(filepath.Join(service.templatePath, "Welcome.html"))
	if err != nil {
		return err
	}

	var data struct {
		ActivationLink string
	}
	data.ActivationLink = link

	err = template.Execute(&buffer, data)
	if err != nil {
		return err
	}

	msg := &m.Message{
		From:    service.sender.From,
		To:      []mail.Address{to},
		Subject: "Activate your mail",
		// TODO(yar): prepare text version of the email
		PlainText: "",
		Parts: []m.Part{
			{
				Type:    "text/html; charset=UTF-8",
				Content: buffer.String(),
			},
		},
	}

	return service.sender.SendEmail(msg)
}

// SendForgotPasswordEmail sends email message with forgot password activation link
// to address should include name of the recipient
func (service *Service) SendForgotPasswordEmail(ctx context.Context, to mail.Address, link string) (err error) {
	defer mon.Task()(&ctx)(&err)
	var buffer bytes.Buffer

	template, err := template.ParseFiles(filepath.Join(service.templatePath, "Forgot.html"))
	if err != nil {
		return err
	}

	var data struct {
		UserName  string
		ResetLink string
	}
	data.UserName = to.Name
	data.ResetLink = link

	err = template.Execute(&buffer, data)
	if err != nil {
		return err
	}

	msg := &m.Message{
		From:    service.sender.From,
		To:      []mail.Address{to},
		Subject: "Forgot password",
		// TODO(yar): prepare text version of the email
		PlainText: "",
		Parts: []m.Part{
			{
				Type:    "text/html; charset=UTF-8",
				Content: buffer.String(),
			},
		},
	}

	return service.sender.SendEmail(msg)
}
