// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"

	goOIDC "github.com/coreos/go-oidc/v3/oidc"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/oauth2"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/console/consoleauth"
)

var (
	// Error is the default error class for the package.
	Error = errs.Class("sso")
	// ErrInvalidProvider is returned when the provider is invalid.
	ErrInvalidProvider = errs.Class("sso:invalid provider")
	// ErrInvalidCode is returned when the auth code is invalid.
	ErrInvalidCode = errs.Class("sso:invalid auth code")
	// ErrNoIdToken is returned when the ID token is missing.
	ErrNoIdToken = errs.Class("sso:missing ID token")
	// ErrTokenVerification is returned when the token verification fails.
	ErrTokenVerification = errs.Class("sso:failed token verification")
	// ErrInvalidState is returned when the state is invalid not what was expected.
	ErrInvalidState = errs.Class("sso:invalid state")
	// ErrInvalidEmail is returned when the email given by sso provider is invalid.
	ErrInvalidEmail = errs.Class("sso:invalid email")
	// ErrInvalidClaims is returned when the claims fail to be parsed.
	ErrInvalidClaims = errs.Class("sso:invalid claims")

	// MicrosoftEntraUrlHost is the host of the Microsoft Entra provider.
	MicrosoftEntraUrlHost = "microsoftonline.com"

	mon = monkit.Package()
)

// Service is a Service for managing SSO.
type Service struct {
	tokens *consoleauth.Service

	config Config

	satelliteAddress string

	providerOidcSetup map[string]OidcSetup

	initialized sync2.Fence
}

// NewService creates a new Service.
func NewService(satelliteAddress string, tokens *consoleauth.Service, config Config) *Service {
	return &Service{
		satelliteAddress: satelliteAddress,
		tokens:           tokens,
		config:           config,
	}
}

// Run runs the OIDC providers initialization.
// NOTE: Run is automatically called by mud framework, but Initialize doesn't.
func (s *Service) Run(ctx context.Context) (err error) {
	return s.Initialize(ctx)
}

// Initialize initializes the OIDC providers.
func (s *Service) Initialize(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !s.config.Enabled {
		return nil
	}

	for _, p := range s.GeneralProviders() {
		if !s.IsProviderConfigured(p) {
			return Error.New("general SSO provider %s is not configured in oidc-provider-infos", p)
		}
	}

	verifierMap := make(map[string]OidcSetup)
	for providerName, info := range s.config.OidcProviderInfos.Values {
		callbackAddr, err := url.JoinPath(s.satelliteAddress, "sso", providerName, "callback")
		if err != nil {
			return Error.Wrap(err)
		}
		var conf OidcConfiguration
		var verifier OidcTokenVerifier
		if s.config.MockSso {
			conf = &MockOidcConfiguration{
				RedirectURL: callbackAddr,
			}
			verifier = &MockVerifier{}
		} else {
			providerUrl := info.ProviderURL.String()
			provider, err := goOIDC.NewProvider(ctx, providerUrl)
			if err != nil {
				return Error.Wrap(err)
			}

			endpoint := provider.Endpoint()
			verifier = provider.Verifier(&goOIDC.Config{ClientID: info.ClientID})

			conf = &oauth2.Config{
				ClientID:     info.ClientID,
				ClientSecret: info.ClientSecret,
				RedirectURL:  callbackAddr,
				Endpoint:     endpoint,
				Scopes:       []string{goOIDC.ScopeOpenID, "email", "profile"},
			}
		}

		verifierMap[providerName] = OidcSetup{
			Url:      info.ProviderURL.String(),
			Config:   conf,
			Verifier: verifier,
		}
	}

	s.providerOidcSetup = verifierMap

	s.initialized.Release()

	return nil
}

// GeneralProviders returns configured general SSO provider names, if any.
func (s *Service) GeneralProviders() []string {
	return s.config.GeneralProviders.Values
}

// IsGeneralProvider returns true if provider matches a general SSO provider.
func (s *Service) IsGeneralProvider(provider string) bool {
	for _, p := range s.config.GeneralProviders.Values {
		if p == provider {
			return true
		}
	}
	return false
}

// IsProviderConfigured returns true if provider exists in oidc-provider-infos.
func (s *Service) IsProviderConfigured(provider string) bool {
	_, ok := s.config.OidcProviderInfos.Values[provider]
	return ok
}

// GeneralLinkVerificationEnabled returns true if general SSO linking requires email verification.
func (s *Service) GeneralLinkVerificationEnabled() bool {
	return s.config.GeneralLinkVerificationEnabled
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
func (s *Service) GetOidcSetupByProvider(ctx context.Context, provider string) *OidcSetup {
	if !s.initialized.Wait(ctx) {
		return nil
	}
	if setup, ok := s.providerOidcSetup[provider]; ok {
		return &setup
	}
	return nil
}

// VerifySso verifies the SSO code as state against a provider.
func (s *Service) VerifySso(ctx context.Context, provider, emailToken, code string) (_ *OidcSsoClaims, err error) {
	defer mon.Task()(&ctx)(&err)

	oidcSetup := s.GetOidcSetupByProvider(ctx, provider)
	if oidcSetup == nil {
		return nil, ErrInvalidProvider.New("invalid provider %s", provider)
	}

	oauth2Token, err := oidcSetup.Config.Exchange(ctx, code)
	if err != nil {
		return nil, ErrInvalidCode.Wrap(err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, ErrNoIdToken.New("missing ID token")
	}

	idToken, err := oidcSetup.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, ErrTokenVerification.Wrap(err)
	}

	var claims OidcSsoClaims
	if s.config.MockSso && s.config.MockEmail != "" {
		claims = OidcSsoClaims{
			Sub:           s.config.MockEmail,
			Email:         s.config.MockEmail,
			EmailVerified: true,
			Name:          "Mock User",
		}
	} else {
		if err = idToken.Claims(&claims); err != nil {
			return nil, ErrInvalidClaims.Wrap(err)
		}
		if strings.Contains(oidcSetup.Url, MicrosoftEntraUrlHost) {
			// For Microsoft Entra, the oid claim is the user's
			// unique identifier. The email claim is not guaranteed
			// the PreferredUsername claim may be the user's email.
			// https://learn.microsoft.com/en-us/entra/identity-platform/id-token-claims-reference
			claims.Sub = claims.Oid
			if claims.Email == "" {
				return nil, ErrInvalidEmail.New("email is empty")
			}
		}
		claims.Email = strings.ToLower(claims.Email)
	}

	if claims.Email == "" {
		return nil, ErrInvalidEmail.New("email is empty")
	}

	if !s.IsGeneralProvider(provider) {
		p := s.GetProviderByEmail(claims.Email)
		if p != provider {
			return nil, ErrInvalidEmail.New("email %s does not match provider %s", claims.Email, provider)
		}

		token, err := s.GetSsoEmailToken(claims.Email)
		if err != nil {
			return nil, Error.New("failed to get email token")
		}
		if emailToken != token {
			return nil, Error.New("invalid email token")
		}
	} else if !claims.EmailVerified {
		return nil, ErrInvalidEmail.New("email is not verified")
	}

	return &claims, nil
}

// GetSsoEmailToken returns a signed string derived from the email address.
func (s *Service) GetSsoEmailToken(email string) (string, error) {
	sum := sha256.Sum256([]byte(email))
	signed, err := s.tokens.Sign(sum[:])
	if err != nil {
		return "", Error.Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(signed), nil
}

// TestSetGeneralLinkVerificationEnabled sets general link verification enabled for testing.
func (s *Service) TestSetGeneralLinkVerificationEnabled(enabled bool) {
	s.config.GeneralLinkVerificationEnabled = enabled
}

// TestSetMockEmail sets the mock email for testing.
func (s *Service) TestSetMockEmail(email string) {
	s.config.MockEmail = email
}
