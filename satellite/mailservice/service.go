// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mailservice

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"path/filepath"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/tenancy"
)

// Config defines values needed by mailservice service.
type Config struct {
	SMTPServerAddress string `help:"smtp server address" default:"" testDefault:"smtp.mail.test:587"`
	TemplatePath      string `help:"path to email templates source" default:""`
	From              string `help:"sender email address" default:"" testDefault:"Labs <storj@mail.test>"`
	AuthType          string `help:"smtp authentication type" releaseDefault:"login" devDefault:"simulate"`
	Login             string `help:"plain/login auth user login" default:""`
	Password          string `help:"plain/login auth user password" default:""`
	RefreshToken      string `help:"refresh token used to retrieve new access token" default:""`
	ClientID          string `help:"oauth2 app's client id" default:""`
	ClientSecret      string `help:"oauth2 app's client secret" default:""`
	TokenURI          string `help:"uri which is used when retrieving new access token" default:""`
}

// WhiteLabelConfig holds tenant-specific branding and SMTP configuration.
type WhiteLabelConfig struct {
	BrandName         string
	LogoURL           string
	HomepageURL       string
	SupportURL        string
	DocsURL           string
	SourceCodeURL     string
	SocialURL         string
	PrivacyPolicyURL  string
	TermsOfServiceURL string
	TermsOfUseURL     string
	BlogURL           string
	CompanyName       string
	AddressLine1      string
	AddressLine2      string
	PrimaryColor      string
}

// TenantConfig holds configuration for multiple tenants.
type TenantConfig struct {
	TenantSenderMap  map[string]Sender
	WhiteLabelConfig map[string]WhiteLabelConfig
}

var (
	mon = monkit.Package()
)

// Sender sends emails.
//
// architecture: Service
type Sender interface {
	SendEmail(ctx context.Context, msg *post.Message) error
	FromAddress() post.Address
}

// Message defines mailservice template-backed message for SendRendered method.
type Message interface {
	Template() string
	Subject() string
}

type emailVars struct {
	WhiteLabelConfig
	// Data is the message-specific data to be used in the template.
	Data any
}

// Service sends template-backed email messages through SMTP.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	Sender Sender

	tenantConfig    TenantConfig
	defaultBranding WhiteLabelConfig

	html *htmltemplate.Template
	// TODO(yar): prepare plain text version
	// text *texttemplate.Template

	sending sync.WaitGroup
}

// New creates new service.
func New(log *zap.Logger, sender Sender, templatePath string, cfg TenantConfig, defaultBranding WhiteLabelConfig) (*Service, error) {
	var err error
	service := &Service{log: log, Sender: sender, tenantConfig: cfg, defaultBranding: defaultBranding}

	// TODO(yar): prepare plain text version
	// service.text, err = texttemplate.ParseGlob(filepath.Join(templatePath, "*.txt"))
	// if err != nil {
	// 	return nil, err
	// }

	service.html, err = htmltemplate.ParseGlob(filepath.Join(templatePath, "*.html"))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return service, nil
}

// Close closes and waits for any pending actions.
func (service *Service) Close() error {
	service.sending.Wait()
	return nil
}

// Send is generalized method for sending custom email message.
func (service *Service) Send(ctx context.Context, msg *post.Message) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.Sender.SendEmail(ctx, msg)
}

// SendRenderedAsync renders content from htmltemplate and texttemplate templates then sends it asynchronously.
func (service *Service) SendRenderedAsync(ctx context.Context, to []post.Address, msg Message) {
	// TODO: think of a better solution
	service.sending.Add(1)
	go func() {
		defer service.sending.Done()

		ctx, cancel := context.WithTimeout(context2.WithoutCancellation(ctx), 5*time.Second)
		defer cancel()

		err := service.SendRendered(ctx, to, msg)

		var recipients []string
		for _, recipient := range to {
			recipients = append(recipients, recipient.String())
		}

		if err != nil {
			service.log.Error("fail sending email",
				zap.String("subject", msg.Subject()),
				zap.Strings("recipients", recipients),
				zap.Error(err))
		} else {
			service.log.Info("email sent successfully",
				zap.String("subject", msg.Subject()),
				zap.Strings("recipients", recipients))
		}
	}()
}

// SendRendered renders content from htmltemplate and texttemplate templates then sends it.
// It merges tenant-specific email variables with the message data before rendering.
func (service *Service) SendRendered(ctx context.Context, to []post.Address, msg Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Get tenant-specific email variables
	templateVars := service.getEmailVars(ctx)
	templateVars.Data = msg

	var htmlBuffer bytes.Buffer
	var textBuffer bytes.Buffer

	// TODO(yar): prepare plain text version
	// if err = service.text.ExecuteTemplate(&textBuffer, msg.Template() + ".txt", msg); err != nil {
	// 	return
	// }

	if err = service.html.ExecuteTemplate(&htmlBuffer, msg.Template()+".html", templateVars); err != nil {
		return err
	}

	sender, err := service.getSenderForTenant(ctx)
	if err != nil {
		return err
	}

	m := &post.Message{
		From:      sender.FromAddress(),
		To:        to,
		Subject:   fmt.Sprintf("%s - %s", templateVars.BrandName, msg.Subject()),
		PlainText: textBuffer.String(),
		Parts: []post.Part{
			{
				Type:    "text/html; charset=UTF-8",
				Content: htmlBuffer.String(),
			},
		},
	}

	err = sender.SendEmail(ctx, m)
	if err != nil {
		tenantID := tenancy.TenantIDFromContext(ctx)
		if tenantID != "" {
			err = errs.Combine(err, errs.New("error sending email for tenant ID: %s", tenantID))
		}
	}

	return err
}

func (service *Service) getEmailVars(ctx context.Context) emailVars {
	defer mon.Task()(&ctx)(nil)

	defaultVars := emailVars{
		WhiteLabelConfig: service.defaultBranding,
	}

	if len(service.tenantConfig.WhiteLabelConfig) == 0 {
		// No config provider - return Storj defaults
		return defaultVars
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	wlCfg := service.tenantConfig.WhiteLabelConfig[tenantID]
	if wlCfg == (WhiteLabelConfig{}) {
		return defaultVars
	}

	return emailVars{
		WhiteLabelConfig: wlCfg,
	}
}

func (service *Service) getSenderForTenant(ctx context.Context) (Sender, error) {
	defer mon.Task()(&ctx)(nil)

	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		return service.Sender, nil
	}

	if sender, exists := service.tenantConfig.TenantSenderMap[tenantID]; exists {
		return sender, nil
	}
	return nil, errs.New("sender not found for tenant ID %s", tenantID)
}

// TestSetTenantSender sets tenant-specific sender for testing purposes.
func (service *Service) TestSetTenantSender(tenantID string, sender Sender) {
	if service.tenantConfig.TenantSenderMap == nil {
		service.tenantConfig.TenantSenderMap = make(map[string]Sender)
	}
	service.tenantConfig.TenantSenderMap[tenantID] = sender
}
