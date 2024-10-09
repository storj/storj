// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso

import (
	"github.com/zeebo/errs"
)

var (
	// Error is the default error class for the package.
	Error = errs.Class("sso")
)

// Service is a service for managing SSO.
type Service struct {
	config Config
}

// NewService creates a new Service.
func NewService(config Config) *Service {
	return &Service{
		config: config,
	}
}
