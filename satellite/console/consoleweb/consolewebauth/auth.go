// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewebauth

import (
	"net/http"
	"time"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
)

// CookieSettings variable cookie settings.
type CookieSettings struct {
	Name string
	Path string
}

// CookieAuth handles cookie authorization.
type CookieAuth struct {
	settings CookieSettings
	domain   string
}

// NewCookieAuth create new cookie authorization with provided settings.
func NewCookieAuth(settings CookieSettings, domain string) *CookieAuth {
	return &CookieAuth{
		settings: settings,
		domain:   domain,
	}
}

// GetToken retrieves token from request.
func (auth *CookieAuth) GetToken(r *http.Request) (console.TokenInfo, error) {
	cookie, err := r.Cookie(auth.settings.Name)
	if err != nil {
		return console.TokenInfo{}, err
	}

	token, err := consoleauth.FromBase64URLString(cookie.Value)
	if err != nil {
		return console.TokenInfo{}, err
	}

	return console.TokenInfo{
		Token:     token,
		ExpiresAt: cookie.Expires,
	}, nil
}

// SetTokenCookie sets parametrized token cookie that is not accessible from js.
func (auth *CookieAuth) SetTokenCookie(w http.ResponseWriter, tokenInfo console.TokenInfo) {
	http.SetCookie(w, &http.Cookie{
		Domain:   auth.domain,
		Name:     auth.settings.Name,
		Value:    tokenInfo.Token.String(),
		Path:     auth.settings.Path,
		Expires:  tokenInfo.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// RemoveTokenCookie removes auth cookie that is not accessible from js.
func (auth *CookieAuth) RemoveTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Domain:   auth.domain,
		Name:     auth.settings.Name,
		Value:    "",
		Path:     auth.settings.Path,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// GetTokenCookieName returns the name of the cookie storing the session token.
func (auth *CookieAuth) GetTokenCookieName() string {
	return auth.settings.Name
}
