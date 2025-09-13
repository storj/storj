// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/uuid"
)

var mon = monkit.Package()

// Config contains configuration parameters for console auth.
type Config struct {
	TokenExpirationTime time.Duration `help:"expiration time for account recovery and activation tokens" default:"10m"`
}

// Service handles creating, signing, and checking the expiration of auth tokens.
type Service struct {
	config Config
	Signer
}

// NewService creates a new consoleauth service.
func NewService(config Config, signer Signer) *Service {
	return &Service{
		config: config,
		Signer: signer,
	}
}

// Signer creates signature for provided data.
type Signer interface {
	Sign(data []byte) ([]byte, error)
}

// CreateToken creates a new auth token.
func (s *Service) CreateToken(ctx context.Context, id uuid.UUID, email string) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)
	claims := &Claims{
		ID:         id,
		Expiration: time.Now().Add(s.config.TokenExpirationTime),
	}
	if email != "" {
		claims.Email = email
	}

	return s.createToken(ctx, claims)
}

// createToken creates string representation.
func (s *Service) createToken(ctx context.Context, claims *Claims) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)

	json, err := claims.JSON()
	if err != nil {
		return "", err
	}

	token := Token{Payload: json}
	signature, err := s.SignToken(token)
	if err != nil {
		return "", err
	}
	token.Signature = signature

	return token.String(), nil
}

// SignToken returns token signature.
func (s *Service) SignToken(token Token) ([]byte, error) {
	encoded := base64.URLEncoding.EncodeToString(token.Payload)

	signature, err := s.Signer.Sign([]byte(encoded))
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// ValidateToken determines token validity using its signature.
func (s *Service) ValidateToken(token Token) (bool, error) {
	signature, err := s.SignToken(token)
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare(signature, token.Signature) == 1, nil
}

// IsExpired returns whether token is expired.
func (s *Service) IsExpired(now, tokenCreatedAt time.Time) bool {
	return now.Sub(tokenCreatedAt) > s.config.TokenExpirationTime
}
