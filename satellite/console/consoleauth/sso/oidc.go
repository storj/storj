// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"context"
	"net/http"
	"net/url"

	goOIDC "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OidcSsoClaims holds info for OIDC token claims.
type OidcSsoClaims struct {
	Sub               string `json:"sub"`
	Oid               string `json:"oid"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
}

// OidcSetup contains the configuration and Verifier
// for an OIDC provider.
type OidcSetup struct {
	Config   OidcConfiguration
	Verifier OidcTokenVerifier
	Url      string
}

// OidcConfiguration is an interface for OIDC configuration.
type OidcConfiguration interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	PasswordCredentialsToken(ctx context.Context, username, password string) (*oauth2.Token, error)
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, t *oauth2.Token) *http.Client
}

// MockOidcConfiguration is a fake OIDC configuration for testing purposes.
type MockOidcConfiguration struct {
	RedirectURL string
}

// AuthCodeURL returns the redirect URL of the satellite with the code and state,
// simulating a successful authentication.
func (c *MockOidcConfiguration) AuthCodeURL(state string, _ ...oauth2.AuthCodeOption) string {
	codeUrl, err := url.Parse(c.RedirectURL)
	if err != nil {
		return ""
	}
	q := codeUrl.Query()
	q.Add("code", "code")
	q.Add("state", state)
	codeUrl.RawQuery = q.Encode()

	return codeUrl.String()
}

// PasswordCredentialsToken simulates the exchange of the username and password for a token.
func (c *MockOidcConfiguration) PasswordCredentialsToken(_ context.Context, _, _ string) (*oauth2.Token, error) {
	return (&oauth2.Token{}).WithExtra(map[string]interface{}{
		"id_token": "extra",
	}), nil
}

// Exchange simulates the exchange of the code for a token.
func (c *MockOidcConfiguration) Exchange(_ context.Context, _ string, _ ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return (&oauth2.Token{}).WithExtra(map[string]interface{}{
		"id_token": "extra",
	}), nil
}

// Client returns a new http client.
func (c *MockOidcConfiguration) Client(_ context.Context, _ *oauth2.Token) *http.Client {
	return &http.Client{}
}

// OidcTokenVerifier is an interface for verifying OIDC tokens.
type OidcTokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*goOIDC.IDToken, error)
}

// MockVerifier is a fake verifier for testing purposes.
type MockVerifier struct{}

// Verify simulates the verification of an OIDC token.
func (v *MockVerifier) Verify(_ context.Context, _ string) (*goOIDC.IDToken, error) {
	return &goOIDC.IDToken{}, nil
}
