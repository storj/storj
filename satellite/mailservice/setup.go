// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package mailservice

import (
	"context"
	"net"
	"net/mail"
	"net/smtp"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/post"
	"storj.io/storj/private/post/oauth2"
)

// TenantSMTPConfig contains tenant-specific SMTP and branding configuration.
type TenantSMTPConfig struct {
	Branding WhiteLabelConfig
	SMTP     Config
}

// SetupConfig contains all configuration needed to set up the mail service.
type SetupConfig struct {
	DefaultSender   Sender
	TemplatePath    string
	TenantConfigs   map[string]TenantSMTPConfig
	DefaultBranding WhiteLabelConfig
}

// SetupWithTenants sets up the mail service with support for multiple tenants.
func SetupWithTenants(log *zap.Logger, cfg SetupConfig) (*Service, error) {
	tenantMailCfg := TenantConfig{
		WhiteLabelConfig: make(map[string]WhiteLabelConfig),
		TenantSenderMap:  make(map[string]Sender),
	}

	for tenantID, tenantCfg := range cfg.TenantConfigs {
		if tenantCfg.SMTP.AuthType != "" && tenantCfg.SMTP.AuthType != "nomail" {
			if err := validateTenantBranding(tenantID, tenantCfg.Branding); err != nil {
				return nil, err
			}
		}

		if err := validateTenantSMTP(tenantID, tenantCfg.SMTP); err != nil {
			return nil, err
		}

		if tenantCfg.SMTP.AuthType != "" {
			sender, err := CreateSender(tenantCfg.SMTP)
			if err != nil {
				return nil, errs.New("failed to create mail sender for tenant ID %s: %v", tenantID, err)
			}
			tenantMailCfg.TenantSenderMap[tenantID] = sender
		}

		tenantMailCfg.WhiteLabelConfig[tenantID] = tenantCfg.Branding
	}

	return New(
		log.Named("mail:service"),
		cfg.DefaultSender,
		cfg.TemplatePath,
		tenantMailCfg,
		cfg.DefaultBranding,
	)
}

// CreateSender creates a mail sender based on the provided configuration.
func CreateSender(mailConfig Config) (Sender, error) {
	switch mailConfig.AuthType {
	case "oauth2":
		creds := oauth2.Credentials{
			ClientID:     mailConfig.ClientID,
			ClientSecret: mailConfig.ClientSecret,
			TokenURI:     mailConfig.TokenURI,
		}
		token, err := oauth2.RefreshToken(context.TODO(), creds, mailConfig.RefreshToken)
		if err != nil {
			return nil, err
		}

		from, err := parseFromAddress(mailConfig.From)
		if err != nil {
			return nil, err
		}

		return &post.SMTPSender{
			From: *from,
			Auth: &oauth2.Auth{
				UserEmail: from.Address,
				Storage:   oauth2.NewTokenStore(creds, *token),
			},
			ServerAddress: mailConfig.SMTPServerAddress,
		}, nil

	case "plain":
		from, host, err := parseFromAndHost(mailConfig)
		if err != nil {
			return nil, err
		}

		return &post.SMTPSender{
			From:          *from,
			Auth:          smtp.PlainAuth("", mailConfig.Login, mailConfig.Password, host),
			ServerAddress: mailConfig.SMTPServerAddress,
		}, nil

	case "login":
		from, err := parseFromAddress(mailConfig.From)
		if err != nil {
			return nil, err
		}

		return &post.SMTPSender{
			From: *from,
			Auth: post.LoginAuth{
				Username: mailConfig.Login,
				Password: mailConfig.Password,
			},
			ServerAddress: mailConfig.SMTPServerAddress,
		}, nil

	case "insecure":
		from, err := parseFromAddress(mailConfig.From)
		if err != nil {
			return nil, err
		}

		return &post.SMTPSender{
			From:          *from,
			ServerAddress: mailConfig.SMTPServerAddress,
		}, nil

	default:
		return nil, errs.New("unsupported auth type: %s", mailConfig.AuthType)
	}
}

func parseFromAndHost(cfg Config) (*mail.Address, string, error) {
	from, err := parseFromAddress(cfg.From)
	if err != nil {
		return nil, "", err
	}

	host, _, err := net.SplitHostPort(cfg.SMTPServerAddress)
	if err != nil && cfg.AuthType != "simulate" && cfg.AuthType != "nologin" {
		return nil, "", errs.New("SMTP server address '%s' couldn't be parsed: %v", cfg.SMTPServerAddress, err)
	}

	return from, host, nil
}

func parseFromAddress(fromAddr string) (*mail.Address, error) {
	from, err := mail.ParseAddress(fromAddr)
	if err != nil {
		return nil, errs.New("SMTP from address '%s' couldn't be parsed: %v", fromAddr, err)
	}
	return from, nil
}

func validateTenantBranding(tenantID string, branding WhiteLabelConfig) error {
	if branding.BrandName == "" {
		return errs.New("missing Name for tenant ID %s", tenantID)
	}
	if branding.LogoURL == "" {
		return errs.New("missing Mail Logo URL for tenant ID %s", tenantID)
	}
	if branding.PrimaryColor == "" {
		return errs.New("missing Primary Color for tenant ID %s", tenantID)
	}
	if branding.HomepageURL == "" {
		return errs.New("missing Homepage URL for tenant ID %s", tenantID)
	}
	if branding.CompanyName == "" || branding.AddressLine1 == "" || branding.AddressLine2 == "" {
		return errs.New("missing Company Name or Address Lines for tenant ID %s", tenantID)
	}
	return nil
}

func validateTenantSMTP(tenantID string, smtp Config) error {
	if smtp.AuthType == "" || smtp.AuthType == "simulated" || smtp.AuthType == "nomail" || smtp.AuthType == "insecure" {
		return nil
	}

	if smtp.AuthType == "oauth2" {
		return errs.New("invalid SMTP Auth Type for tenant ID %s", tenantID)
	}

	if smtp.Login == "" || smtp.Password == "" {
		return errs.New("missing SMTP Login or Password for tenant ID %s", tenantID)
	}

	return nil
}
