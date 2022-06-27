// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewebauth

import (
	"net/http"
	"time"

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
}

// NewCookieAuth create new cookie authorization with provided settings.
func NewCookieAuth(settings CookieSettings) *CookieAuth {
	return &CookieAuth{
		settings: settings,
	}
}

// GetToken retrieves token from request.
func (auth *CookieAuth) GetToken(r *http.Request) (consoleauth.Token, error) {
	cookie, err := r.Cookie(auth.settings.Name)
	if err != nil {
		return consoleauth.Token{}, err
	}

	token, err := consoleauth.FromBase64URLString(cookie.Value)
	if err != nil {
		return consoleauth.Token{}, err
	}

	return token, nil
}

// SetTokenCookie sets parametrized token cookie that is not accessible from js.
func (auth *CookieAuth) SetTokenCookie(w http.ResponseWriter, token consoleauth.Token) {
	http.SetCookie(w, &http.Cookie{
		Name:  auth.settings.Name,
		Value: token.String(),
		Path:  auth.settings.Path,
		// TODO: get expiration from token
		Expires:  time.Now().Add(time.Hour * 24),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// RemoveTokenCookie removes auth cookie that is not accessible from js.
func (auth *CookieAuth) RemoveTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
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
