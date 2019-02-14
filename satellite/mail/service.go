// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import (
	"bytes"
	"context"
	html "html/template"
	"io"
	"net/mail"
	text "text/template"

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

// FromAddress parses email address from config to mail.Address
func (c *Config) FromAddress() (*mail.Address, error) {
	return mail.ParseAddress(c.From)
}

var (
	mon = monkit.Package()
)

// Template defines mail template for SendRendered method
type Template interface {
	To() []mail.Address
	Subject() string
	HTMLPath() string
	PainTextPath() string
}

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

// Send is generalized method for sending custom email message
func (service *Service) Send(ctx context.Context, msg *m.Message) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.sender.SendEmail(msg)
}

// SendRendered renders content from html and text templates then sends it
func (service *Service) SendRendered(ctx context.Context, tmpl Template) (err error) {
	defer mon.Task()(&ctx)(&err)

	var htmlBuffer bytes.Buffer
	var textBuffer bytes.Buffer

	// render text template
	if err = RenderPlainText(&textBuffer, tmpl); err != nil {
		return
	}

	// render html template
	if err = RenderHTML(&htmlBuffer, tmpl); err != nil {
		return
	}

	msg := &m.Message{
		From:      service.sender.From,
		To:        tmpl.To(),
		Subject:   tmpl.Subject(),
		PlainText: textBuffer.String(),
		Parts: []m.Part{
			{
				Type:    "text/html; charset=UTF-8",
				Content: htmlBuffer.String(),
			},
		},
	}

	return service.sender.SendEmail(msg)
}

// RenderHTML renders html content of given Template and writes it to writer
func RenderHTML(w io.Writer, tmpl Template) error {
	template, err := html.ParseFiles(tmpl.HTMLPath())
	if err != nil {
		return err
	}

	if err = template.Execute(w, tmpl); err != nil {
		return err
	}

	return nil
}

// RenderPlainText renders text content of given Template and writes it to writer
func RenderPlainText(w io.Writer, tmpl Template) error {
	template, err := text.ParseFiles(tmpl.PainTextPath())
	if err != nil {
		return err
	}

	if err = template.Execute(w, tmpl); err != nil {
		return err
	}

	return nil
}
