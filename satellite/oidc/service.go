// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

// NewService constructs a service for handling various OAuth and OIDC operations.
func NewService(db DB) *Service {
	return &Service{
		store: db,
	}
}

// Service provides common implementations for managing clients and tokens.
//
// architecture: Service
type Service struct {
	store DB
}

// ClientStore returns a store used to lookup oauth clients from the consent flow.
func (s *Service) ClientStore() *ClientStore {
	return &ClientStore{
		clients: s.store.OAuthClients(),
	}
}

// TokenStore returns a store used to manage access tokens during the consent flow.
func (s *Service) TokenStore() *TokenStore {
	return &TokenStore{
		codes:  s.store.OAuthCodes(),
		tokens: s.store.OAuthTokens(),
	}
}
