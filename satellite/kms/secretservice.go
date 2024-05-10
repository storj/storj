// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"

	"storj.io/common/storj"
)

// SecretsService is a service for retrieving the master key.
//
// architecture: Service
type SecretsService interface {
	// Initialize gets and validates the master key.
	Initialize(ctx context.Context) error
	// getMasterKey returns the master key.
	getMasterKey() (*storj.Key, error)
	// Close closes the service.
	Close() error
}
