// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation

import "context"

// DB is the interface for a revocation DB.
type DB interface {
	// Revoke is the method to revoke the supplied tail
	Revoke(ctx context.Context, tail []byte, apiKeyID []byte) error
	// Check will check whether any of the supplied tails have been revoked
	Check(ctx context.Context, tails [][]byte) (bool, error)
}
