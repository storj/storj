// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"context"
	"net/url"

	goOIDC "github.com/coreos/go-oidc/v3/oidc"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/oauth2"
)

var (
	// Error is the default error class for the package.
	Error = errs.Class("sso")

	mon = monkit.Package()
)

// OidcSsoClaims holds info for OIDC token claims.
type OidcSsoClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// OidcSetup contains the configuration and verifier
// for an OIDC provider.
type OidcSetup struct {
	Config   oauth2.Config
	Verifier *goOIDC.IDTokenVerifier
}

// Service is a Service for managing SSO.
type Service struct {
	config Config

	satelliteAddress string

	providerOidcSetup map[string]OidcSetup
}

// NewService creates a new Service.
func NewService(satelliteAddress string, config Config) *Service {
	return &Service{
		satelliteAddress: satelliteAddress,
		config:           config,
	}
}

// Initialize initializes the OIDC providers.
func (s *Service) Initialize(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	verifierMap := make(map[string]OidcSetup)
	for providerName, info := range s.config.OidcProviderInfos.Values {
		callbackAddr, err := url.JoinPath(s.satelliteAddress, "sso", providerName, "callback")
		if err != nil {
			return Error.Wrap(err)
		}
		conf := oauth2.Config{
			ClientID:     info.ClientID,
			ClientSecret: info.ClientSecret,
			RedirectURL:  callbackAddr,
			Endpoint: oauth2.Endpoint{
				AuthURL:  info.ProviderURL.String() + "/oauth2/v1/authorize",
				TokenURL: info.ProviderURL.String() + "/oauth2/v1/token",
			},
			Scopes: []string{goOIDC.ScopeOpenID, "email", "profile"},
		}

		provider, err := goOIDC.NewProvider(ctx, info.ProviderURL.String())
		if err != nil {
			return Error.Wrap(err)
		}
		verifier := provider.Verifier(&goOIDC.Config{ClientID: conf.ClientID})
		if verifier == nil {
			return Error.New("failed to create Verifier")
		}
		verifierMap[providerName] = OidcSetup{
			Config:   conf,
			Verifier: verifier,
		}
	}

	s.providerOidcSetup = verifierMap

	return nil
}

// InitializeRoutes provides a routingFn with configured providers
// to configure the routes for sso.
func (s *Service) InitializeRoutes(routingFn func(provider string)) {
	for provider := range s.config.EmailProviderMappings.Values {
		routingFn(provider)
	}
}

// GetProviderByEmail returns the provider for the given email.
func (s *Service) GetProviderByEmail(email string) string {
	for provider, emailRegex := range s.config.EmailProviderMappings.Values {
		if emailRegex.MatchString(email) {
			return provider
		}
	}
	return ""
}

// GetOidcSetupByProvider returns the OIDC setup for the given provider.
func (s *Service) GetOidcSetupByProvider(provider string) *OidcSetup {
	if setup, ok := s.providerOidcSetup[provider]; ok {
		return &setup
	}
	return nil
}
