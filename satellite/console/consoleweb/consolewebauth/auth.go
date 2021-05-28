// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewebauth

import (
	"net/http"
	"time"
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
func (auth *CookieAuth) GetToken(r *http.Request) (string, error) {
	cookie, err := r.Cookie(auth.settings.Name)
	if err != nil {
		return "", err
	}

	return cookie.Value, nil
}

// SetTokenCookie sets parametrized token cookie that is not accessible from js.
func (auth *CookieAuth) SetTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:  auth.settings.Name,
		Value: token,
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
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
