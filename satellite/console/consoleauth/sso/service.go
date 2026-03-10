// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	goOIDC "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
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
	// ErrWebhookUnauthorized is returned when webhook request authentication fails.
	ErrWebhookUnauthorized = errs.Class("sso:webhook unauthorized")
	// ErrWebhookBadRequest is returned when a webhook request body cannot be parsed.
	ErrWebhookBadRequest = errs.Class("sso:webhook bad request")

	// MicrosoftEntraUrlHost is the host of the Microsoft Entra provider.
	MicrosoftEntraUrlHost = "microsoftonline.com"

	mon = monkit.Package()
)

// WebhookEventTypeUserUpdate is the event type sent by the auth provider
// when a user's profile is updated.
const WebhookEventTypeUserUpdate = "user.update.complete"

// WebhookUserData holds the user fields in a webhook event.
type WebhookUserData struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Verified bool   `json:"verified"`
}

// WebhookEvent is a parsed webhook event.
type WebhookEvent struct {
	Type string          `json:"type"`
	User WebhookUserData `json:"user"`
}

type webhookPayload struct {
	Event WebhookEvent `json:"event"`
}

type webhookSignatureClaims struct {
	RequestBodySHA256 string `json:"request_body_sha256"`
	jwt.RegisteredClaims
}

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

	if s.config.PrimaryAuthProvider != "" && !s.IsProviderConfigured(s.config.PrimaryAuthProvider) {
		return Error.New("primary auth provider %s is not configured in oidc-provider-infos", s.config.PrimaryAuthProvider)
	}

	wh := s.config.Webhook
	if wh.Enabled {
		if wh.Username == "" || wh.Password == "" || wh.SigningSecret == "" || wh.SignatureHeader == "" {
			return Error.New("webhook is enabled; username, password, signing-secret, and signature-header must all be provided")
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
		var logoutURL string
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

			var providerClaims struct {
				EndSessionEndpoint string `json:"end_session_endpoint"`
			}
			if claimErr := provider.Claims(&providerClaims); claimErr == nil {
				// see https://openid.net/specs/openid-connect-rpinitiated-1_0.html
				endSessionEndpoint, err := url.Parse(providerClaims.EndSessionEndpoint)
				if err != nil {
					return Error.Wrap(err)
				}
				params := endSessionEndpoint.Query()
				params.Add("client_id", info.ClientID)
				endSessionEndpoint.RawQuery = params.Encode()
				logoutURL = endSessionEndpoint.String()
			} else {
				// best effort to construct logout url. unlikely to happen but just in case
				// should be in the form https://{provider-host}/oauth2/logout?client_id={client-id}
				customEndSessionURL := info.ProviderURL
				customEndSessionURL.Path = "oauth2/logout"
				params := customEndSessionURL.Query()
				params.Add("client_id", info.ClientID)
				customEndSessionURL.RawQuery = params.Encode()
				logoutURL = customEndSessionURL.String()
			}

			conf = &oauth2.Config{
				ClientID:     info.ClientID,
				ClientSecret: info.ClientSecret,
				RedirectURL:  callbackAddr,
				Endpoint:     endpoint,
				Scopes:       []string{goOIDC.ScopeOpenID, "email", "profile"},
			}
		}

		verifierMap[providerName] = OidcSetup{
			Url:       info.ProviderURL.String(),
			LogoutURL: logoutURL,
			Config:    conf,
			Verifier:  verifier,
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

// IsPrimaryAuthProvider returns true if provider is the primary auth provider.
func (s *Service) IsPrimaryAuthProvider(provider string) bool {
	return s.config.PrimaryAuthProvider == provider
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

// GetLogoutURL returns the logout URL for the given provider.
// It returns an empty string if the provider is not found or if the service is not initialized.
func (s *Service) GetLogoutURL(ctx context.Context, provider string) string {
	if !s.initialized.Wait(ctx) {
		return ""
	}
	if setup, ok := s.providerOidcSetup[provider]; ok {
		return setup.LogoutURL
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
// It returns the claims, the provider access token and its expiry.
func (s *Service) VerifySso(ctx context.Context, provider, emailToken, code string) (_ *OidcSsoClaims, accessToken string, expiry time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	oidcSetup := s.GetOidcSetupByProvider(ctx, provider)
	if oidcSetup == nil {
		return nil, "", time.Time{}, ErrInvalidProvider.New("invalid provider %s", provider)
	}

	oauth2Token, err := oidcSetup.Config.Exchange(ctx, code)
	if err != nil {
		return nil, "", time.Time{}, ErrInvalidCode.Wrap(err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, "", time.Time{}, ErrNoIdToken.New("missing ID token")
	}

	idToken, err := oidcSetup.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", time.Time{}, ErrTokenVerification.Wrap(err)
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
			return nil, "", time.Time{}, ErrInvalidClaims.Wrap(err)
		}
		if strings.Contains(oidcSetup.Url, MicrosoftEntraUrlHost) {
			// For Microsoft Entra, the oid claim is the user's
			// unique identifier. The email claim is not guaranteed
			// the PreferredUsername claim may be the user's email.
			// https://learn.microsoft.com/en-us/entra/identity-platform/id-token-claims-reference
			claims.Sub = claims.Oid
			if claims.Email == "" {
				return nil, "", time.Time{}, ErrInvalidEmail.New("email is empty")
			}
		}
		claims.Email = strings.ToLower(claims.Email)
	}

	if claims.Email == "" {
		return nil, "", time.Time{}, ErrInvalidEmail.New("email is empty")
	}

	if !s.IsGeneralProvider(provider) {
		p := s.GetProviderByEmail(claims.Email)
		if p != provider {
			return nil, "", time.Time{}, ErrInvalidEmail.New("email %s does not match provider %s", claims.Email, provider)
		}

		token, err := s.GetSsoEmailToken(claims.Email)
		if err != nil {
			return nil, "", time.Time{}, Error.New("failed to get email token")
		}
		if emailToken != token {
			return nil, "", time.Time{}, Error.New("invalid email token")
		}
	} else if !s.config.AllowUnverifiedGeneralSSO && !claims.EmailVerified {
		return nil, "", time.Time{}, ErrInvalidEmail.New("email is not verified")
	}

	if !s.IsPrimaryAuthProvider(provider) {
		return &claims, "", time.Time{}, nil
	}

	return &claims, oauth2Token.AccessToken, oauth2Token.Expiry, nil
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

// ValidateAndParseWebhookData authenticates the request using the configured Basic Auth credentials
// and HMAC signing secret, then parses the body into a WebhookEvent.
func (s *Service) ValidateAndParseWebhookData(username, password, sigToken string, body []byte) (WebhookEvent, error) {
	whCfg := s.config.Webhook

	if username == "" && password == "" {
		return WebhookEvent{}, ErrWebhookUnauthorized.New("missing basic auth credentials")
	}
	if subtle.ConstantTimeCompare([]byte(username), []byte(whCfg.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(whCfg.Password)) != 1 {
		return WebhookEvent{}, ErrWebhookUnauthorized.New("basic auth credentials mismatch")
	}
	if sigToken == "" {
		return WebhookEvent{}, ErrWebhookUnauthorized.New("missing signature")
	}

	claims := &webhookSignatureClaims{}
	_, err := jwt.ParseWithClaims(sigToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, Error.New("unexpected JWT signing method: %v", token.Header["alg"])
		}
		return []byte(whCfg.SigningSecret), nil
	})
	if err != nil {
		return WebhookEvent{}, ErrWebhookUnauthorized.Wrap(err)
	}

	bodyHash := sha256.Sum256(body)
	if claims.RequestBodySHA256 != base64.StdEncoding.EncodeToString(bodyHash[:]) {
		return WebhookEvent{}, ErrWebhookUnauthorized.New("request body hash mismatch")
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, ErrWebhookBadRequest.Wrap(err)
	}

	return payload.Event, nil
}

// PrimaryAuthProvider returns the name of the primary SSO auth provider, or "" if not configured.
func (s *Service) PrimaryAuthProvider() string {
	return s.config.PrimaryAuthProvider
}

// GetAccountURL returns the account management URL for the primary auth provider.
func (s *Service) GetAccountURL() string {
	return s.config.AccountURL
}

// WebhookEnabled returns whether the primary auth provider webhook is enabled.
func (s *Service) WebhookEnabled() bool {
	return s.config.Webhook.Enabled
}

// WebhookSignatureHeader returns the name of the header containing the webhook signature.
func (s *Service) WebhookSignatureHeader() string {
	return s.config.Webhook.SignatureHeader
}

// TestSetGeneralLinkVerificationEnabled sets general link verification enabled for testing.
func (s *Service) TestSetGeneralLinkVerificationEnabled(enabled bool) {
	s.config.GeneralLinkVerificationEnabled = enabled
}

// TestSetMockEmail sets the mock email for testing.
func (s *Service) TestSetMockEmail(email string) {
	s.config.MockEmail = email
}

// TestSetMockAccessToken configures the mock access token on all mock providers.
func (s *Service) TestSetMockAccessToken(token string) {
	for _, setup := range s.providerOidcSetup {
		if mock, ok := setup.Config.(*MockOidcConfiguration); ok {
			mock.MockAccessToken = token
		}
	}
}
