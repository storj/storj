// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"

	goOIDC "github.com/coreos/go-oidc/v3/oidc"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/oauth2"

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
	// ErrInvalidClaims is returned when the claims fail to be parsed.
	ErrInvalidClaims = errs.Class("sso:invalid claims")

	mon = monkit.Package()
)

// Service is a Service for managing SSO.
type Service struct {
	tokens *consoleauth.Service

	config Config

	satelliteAddress string

	providerOidcSetup map[string]OidcSetup
}

// NewService creates a new Service.
func NewService(satelliteAddress string, tokens *consoleauth.Service, config Config) *Service {
	return &Service{
		satelliteAddress: satelliteAddress,
		tokens:           tokens,
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
		var conf OidcConfiguration
		var verifier OidcTokenVerifier
		if s.config.MockSso {
			conf = &MockOidcConfiguration{
				RedirectURL: callbackAddr,
			}
			verifier = &MockVerifier{}
		} else {
			conf = &oauth2.Config{
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
			v := provider.Verifier(&goOIDC.Config{ClientID: info.ClientID})
			if v == nil {
				return Error.New("failed to create Verifier")
			}
			verifier = v
		}

		verifierMap[providerName] = OidcSetup{
			Config:   conf,
			Verifier: verifier,
		}
	}

	s.providerOidcSetup = verifierMap

	return nil
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

// VerifySso verifies the SSO code as state against a provider.
func (s *Service) VerifySso(ctx context.Context, provider, state, code string) (_ *OidcSsoClaims, err error) {
	defer mon.Task()(&ctx)(&err)

	oidcSetup := s.GetOidcSetupByProvider(provider)
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
			Sub:   s.config.MockEmail,
			Email: s.config.MockEmail,
			Name:  "Mock User",
		}
	} else {
		if err = idToken.Claims(&claims); err != nil {
			return nil, ErrInvalidClaims.Wrap(err)
		}
	}

	stat, err := s.GetSsoStateFromEmail(claims.Email)
	if err != nil {
		return nil, Error.New("failed to get state")
	}
	if state != stat {
		return nil, ErrInvalidState.New("state mismatch")
	}

	return &claims, nil
}

// GetSsoStateFromEmail returns a signed string derived from the email address.
func (s *Service) GetSsoStateFromEmail(email string) (string, error) {
	sum := sha256.Sum256([]byte(email))
	signed, err := s.tokens.Sign(sum[:])
	if err != nil {
		return "", Error.Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(signed), nil
}
