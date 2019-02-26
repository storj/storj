// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mailservice

import (
	"bytes"
	"context"
	htmltemplate "html/template"
	"path/filepath"
	texttemplate "text/template"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/post"
)

// Config defines values needed by mailservice service
type Config struct {
	SMTPServerAddress string `help:"smtp server address" default:""`
	TemplatePath      string `help:"path to email templates source" default:""`
	From              string `help:"sender email address" default:""`
	Auth              AuthConfig
}

// AuthConfig SMTP auth configuration
type AuthConfig struct {
	Type   string `help:"smtp authentication type" default:"OAuth2"`
	Plain  PlainAuth
	OAuth2 OAuth2
}

// PlainAuth SMTP configuration
type PlainAuth struct {
	Login    string `help:"plain auth user login" default:""`
	Password string `help:"plain auth user password" default:""`
}

// OAuth2 SMTP configuration
type OAuth2 struct {
	RefreshToken string `help:"refresh token used to retrieve new access token" default:""`
	Credentials  OAuth2Credentials
}

// OAuth2Credentials is OAuth2 app credentials
type OAuth2Credentials struct {
	ClientID     string `help:"oauth2 app's client id" default:""`
	ClientSecret string `help:"oauth2 app's client secret" default:""`
	TokenURI     string `help:"uri which is used when retrieving new access token" default:""`
}

var (
	mon = monkit.Package()
)

// Sender is
type Sender interface {
	SendEmail(msg *post.Message) error
	FromAddress() post.Address
}

// Message defines mailservice template-backed message for SendRendered method
type Message interface {
	Template() string
	Subject() string
}

// Service sends template-backed email messages through SMTP
type Service struct {
	log    *zap.Logger
	sender Sender

	html *htmltemplate.Template
	text *texttemplate.Template
}

// New creates new service
func New(log *zap.Logger, sender Sender, templatePath string) (*Service, error) {
	var err error
	service := &Service{log: log, sender: sender}

	// TODO(yar): prepare plain text version
	//service.text, err = texttemplate.ParseGlob(filepath.Join(templatePath, "*.txt"))
	//if err != nil {
	//	return nil, err
	//}

	service.html, err = htmltemplate.ParseGlob(filepath.Join(templatePath, "*.html"))
	if err != nil {
		return nil, err
	}

	return service, nil
}

// Send is generalized method for sending custom email message
func (service *Service) Send(ctx context.Context, msg *post.Message) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.sender.SendEmail(msg)
}

// SendRendered renders content from htmltemplate and texttemplate templates then sends it
func (service *Service) SendRendered(ctx context.Context, to []post.Address, msg Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	var htmlBuffer bytes.Buffer
	var textBuffer bytes.Buffer

	// TODO(yar): prepare plain text version
	//if err = service.text.ExecuteTemplate(&textBuffer, msg.Template() + ".txt", msg); err != nil {
	//	return
	//}

	if err = service.html.ExecuteTemplate(&htmlBuffer, msg.Template()+".html", msg); err != nil {
		return
	}

	m := &post.Message{
		From:      service.sender.FromAddress(),
		To:        to,
		Subject:   msg.Subject(),
		PlainText: textBuffer.String(),
		Parts: []post.Part{
			{
				Type:    "text/html; charset=UTF-8",
				Content: htmlBuffer.String(),
			},
		},
	}

	err = service.sender.SendEmail(m)
	// log error
	if err != nil {
		service.log.Info("error from mail sender", zap.String("error", err.Error()))
	} else {
		service.log.Info("successfully send message")
	}

	return err
}
