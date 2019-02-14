// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mailservice

import (
	"bytes"
	"context"
	htmltemplate "html/template"
	"io"
	texttemplate "text/template"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/post"
)

// Config defines values needed by mailservice service
type Config struct {
	SMTPServerAddress string `help:"smtp server address" default:""`
	TemplatePath      string `help:"path to mailservice templates source" default:""`
	From              string `help:"sender email address" default:""`
	Auth              struct {
		Type  string `help:"smtp authentication type" default:"OAuth2"`
		Plain struct {
			Login    string `help:"plain auth user login" default:""`
			Password string `help:"plain auth user password" default:""`
		}
		OAuth2 struct {
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

// Template defines mailservice template for SendRendered method
type Template interface {
	To() []post.Address
	Subject() string
	HTMLPath() string
	PainTextPath() string
}

// Service sends predefined email messages through SMTP
type Service struct {
	log    *zap.Logger
	sender post.SMTPSender

	templatePath string
}

// New creates new service
func New(log *zap.Logger, sender post.SMTPSender, templatePath string) *Service {
	return &Service{log: log, sender: sender, templatePath: templatePath}
}

// Send is generalized method for sending custom email message
func (service *Service) Send(ctx context.Context, msg *post.Message) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.sender.SendEmail(msg)
}

// SendRendered renders content from htmltemplate and texttemplate templates then sends it
func (service *Service) SendRendered(ctx context.Context, tmpl Template) (err error) {
	defer mon.Task()(&ctx)(&err)

	var htmlBuffer bytes.Buffer
	var textBuffer bytes.Buffer

	// render texttemplate template
	if err = RenderPlainText(&textBuffer, tmpl); err != nil {
		return
	}

	// render htmltemplate template
	if err = RenderHTML(&htmlBuffer, tmpl); err != nil {
		return
	}

	msg := &post.Message{
		From:      service.sender.From,
		To:        tmpl.To(),
		Subject:   tmpl.Subject(),
		PlainText: textBuffer.String(),
		Parts: []post.Part{
			{
				Type:    "texttemplate/htmltemplate; charset=UTF-8",
				Content: htmlBuffer.String(),
			},
		},
	}

	return service.sender.SendEmail(msg)
}

// RenderHTML renders htmltemplate content of given Template and writes it to writer
func RenderHTML(w io.Writer, tmpl Template) error {
	template, err := htmltemplate.ParseFiles(tmpl.HTMLPath())
	if err != nil {
		return err
	}

	if err = template.Execute(w, tmpl); err != nil {
		return err
	}

	return nil
}

// RenderPlainText renders texttemplate content of given Template and writes it to writer
func RenderPlainText(w io.Writer, tmpl Template) error {
	template, err := texttemplate.ParseFiles(tmpl.PainTextPath())
	if err != nil {
		return err
	}

	if err = template.Execute(w, tmpl); err != nil {
		return err
	}

	return nil
}
