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
	// GetKeys gets key from the source.
	GetKeys(ctx context.Context) (map[int]*storj.Key, error)
	// Close closes the service.
	Close() error
}
