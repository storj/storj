// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"

	"storj.io/common/storj"
)

// SecretsService is a service for retrieving keys.
//
// architecture: Service
type SecretsService interface {
	// GetKey gets a key from the secret service.
	GetKey(ctx context.Context, keyInfo KeyInfo) (*storj.Key, error)
	// Close closes the service.
	Close() error
}
