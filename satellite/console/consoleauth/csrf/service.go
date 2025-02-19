// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package csrf

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"storj.io/storj/satellite/console/consoleauth"
)

// CookieName is the name of the cookie storing the CSRF token.
const CookieName = "csrf_token"

// Service provides security token generation and CSRF cookie setting.
type Service struct {
	signer consoleauth.Signer
}

// NewService creates a new CSRF service.
func NewService(signer consoleauth.Signer) *Service {
	return &Service{signer: signer}
}

// GenerateSecurityToken generates a random signed security token.
func (s *Service) GenerateSecurityToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	token := consoleauth.Token{Payload: b}
	encoded := base64.URLEncoding.EncodeToString(token.Payload)

	signature, err := s.signer.Sign([]byte(encoded))
	if err != nil {
		return "", err
	}

	token.Signature = signature

	return token.String(), nil
}

// SetCookie sets parametrized CSRF cookie that is not accessible from js.
func (s *Service) SetCookie(w http.ResponseWriter) error {
	token, err := s.GenerateSecurityToken()
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	return nil
}
